//
// FilePath    : zap-smap\process.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 处理单个文件的 AST 修改逻辑
//

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jiaopengzi/go-utils"
)

// processFile 负责解析单个文件, 执行 AST 修改并返回是否修改、修改后的源码和发生修改的行号列表
func processFile(path string, fSet *token.FileSet, modulePath string, baseDir string) (bool, string, []int, error) {
	// 读取文件内容
	src, err := utils.ReadFile(path)
	if err != nil {
		return false, "", nil, err
	}

	// 解析文件为 AST
	file, err := parser.ParseFile(fSet, path, src, parser.ParseComments)
	if err != nil {
		// 解析失败时至少记录警告, 避免静默跳过
		fmt.Fprintf(os.Stderr, "warn: parse %s failed: %v\n", path, err)
		return false, "", nil, nil
	}

	// 判断是否包含 zap 导入
	if !hasZapImport(file) {
		return false, "", nil, nil
	}

	modified := false

	// 收集函数范围信息
	fns := collectFuncRanges(file, fSet)

	var modifiedLines []int

	// 通过 ast.Inspect 遍历 AST 节点
	ast.Inspect(file, func(n ast.Node) bool {
		// 处理 CallExpr 节点
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// 处理 SelectorExpr 函数调用
		sel, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// 将复杂逻辑委托给 handleCallExpr, 便于拆分和测试
		if ok, line := handleCallExpr(ce, sel, fSet, file, fns, modulePath, baseDir); ok {
			modified = true

			modifiedLines = append(modifiedLines, line)
		}

		return true
	})

	// 如果文件被修改, 则生成修改后的源码
	if modified {
		var sb strings.Builder
		if err := printer.Fprint(&sb, fSet, file); err != nil {
			return false, "", nil, err
		}

		out := sb.String()

		// 二次修正: go/printer 可能重排代码行(如 CompositeLit 被拆行),
		// 导致注入的行号与实际行号不符。重新解析输出, 校正行号。
		if *delFlg == "" {
			out = correctLineNumbers(out, path, modulePath, baseDir)
		}

		return true, out, modifiedLines, nil
	}

	return false, "", nil, nil
}

// handleCallExpr 处理单个 CallExpr, 返回是否修改以及修改所在的行号
func handleCallExpr(ce *ast.CallExpr, sel *ast.SelectorExpr, fSet *token.FileSet, file *ast.File, fns []fnRange, modulePath, baseDir string) (bool, int) {
	method := sel.Sel.Name
	if !logMethods[method] {
		return false, 0
	}

	if !isZapChain(sel.X) {
		return false, 0
	}

	if len(ce.Args) == 0 {
		return false, 0
	}

	// 如果指定了要删除的字段, 执行纯删除操作后立即返回, 不再注入新字段
	if *delFlg != "" {
		deleted := false

		if ce.Ellipsis.IsValid() {
			// ellipsis 调用: 检查展开参数是否被 append([]zap.Field{zap.String(delKey, ...)}, x...) 包裹, 解包还原
			lastIdx := len(ce.Args) - 1
			if _, _, origArg := findEllipsisFieldCall(ce.Args[lastIdx], *delFlg); origArg != nil {
				ce.Args[lastIdx] = origArg
				deleted = true
			}
		} else {
			if idxDel := findExistingFieldIndex(ce, *delFlg); idxDel >= 0 {
				if idxDel >= 0 && idxDel < len(ce.Args) {
					ce.Args = append(ce.Args[:idxDel], ce.Args[idxDel+1:]...)
					deleted = true
				}
			}
		}

		if deleted {
			pos := fSet.Position(ce.Lparen)
			return true, pos.Line
		}

		return false, 0
	}

	// 使用 analyzeCallExpr 收集共享信息
	isTarget, pos, _, _, _, expected, foundIndex := analyzeCallExpr(ce, sel, fSet, file, fns, modulePath, baseDir)
	if !isTarget {
		return false, 0
	}

	// ellipsis 路径: 使用 append([]zap.Field{zap.String("fl", "...")}, expandedArg...) 包裹
	// 确保注入字段在切片第一位
	if ce.Ellipsis.IsValid() {
		lastIdx := len(ce.Args) - 1
		expandedArg := ce.Args[lastIdx]

		// 检查是否已包裹: append([]zap.Field{zap.String("fl", "...")}, x...) → 更新值
		if _, zapCall, _ := findEllipsisFieldCall(expandedArg, *fieldFlg); zapCall != nil {
			if len(zapCall.Args) >= 2 {
				oldPos := zapCall.Args[1].Pos()
				zapCall.Args[1] = &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(expected), ValuePos: oldPos}
			}

			return true, pos.Line
		}

		// 未包裹: 用 append([]zap.Field{newArg}, expandedArg...) 包裹, 注入字段在切片第一位
		newArg := makeZapStringArg(expected)
		wrapExpr := makeEllipsisAppend(newArg, expandedArg)
		ce.Args[lastIdx] = wrapExpr

		return true, pos.Line
	}

	// 非 ellipsis 路径: 直接插入或更新参数
	newArg := makeZapStringArg(expected)

	// 设置新节点位置, 防止 go/printer 将相邻注释吸入参数内部
	setExprPos(newArg, ce.Lparen)

	if foundIndex >= 0 {
		ce.Args[foundIndex] = newArg
	} else {
		old := ce.Args

		// 计算插入索引: position 基于 field 参数列表(跳过第一个 msg 参数)
		// position=0 表示插入到第一个 field 之前(即 msg 之后), 默认(-1)等同于 0
		insertIdx := 1 // AST 索引 1 = msg 之后

		if *positionFlg >= 0 {
			// field 索引 + 1(跳过 msg) = AST 索引
			astIdx := *positionFlg + 1
			if astIdx > len(old) {
				insertIdx = len(old)
			} else {
				insertIdx = astIdx
			}
		}

		// 插入 newArg 到 insertIdx
		switch {
		case insertIdx <= 0:
			newArgs := make([]ast.Expr, 0, len(old)+1)
			newArgs = append(newArgs, newArg)
			newArgs = append(newArgs, old...)
			ce.Args = newArgs
		case insertIdx >= len(old):
			old = append(old, newArg)
			ce.Args = old
		default:
			newArgs := make([]ast.Expr, 0, len(old)+1)
			newArgs = append(newArgs, old[:insertIdx]...)
			newArgs = append(newArgs, newArg)
			newArgs = append(newArgs, old[insertIdx:]...)
			ce.Args = newArgs
		}

		// 如果要求按字母排序 zap 字段，则重建参数列表：保持第一个参数(通常是消息)
		// 然后将所有 zap 字段按照 key 排序，剩余非 zap 字段保持原序追加
		if *sortFlg {
			sortZapFields(ce)
		}
	}

	return true, pos.Line
}

// makeZapStringArg 构造 zap.String(...) 表达式
func makeZapStringArg(v string) ast.Expr {
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{X: ast.NewIdent(zapIdent), Sel: ast.NewIdent("String")},
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(*fieldFlg)},
			&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(v)},
		},
	}
}

// findExistingFieldIndex 在已有参数中查找是否已经包含目标字段, 返回真实索引或 -1
func findExistingFieldIndex(ce *ast.CallExpr, key string) int {
	// 遍历传入参数(跳过第一个参数), 使用短路 continue 降低嵌套层级
	for i, a := range ce.Args[1:] {
		call, ok := a.(*ast.CallExpr)
		if !ok {
			continue
		}

		funSel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		id, ok := funSel.X.(*ast.Ident)
		if !ok || id.Name != zapIdent {
			continue
		}

		// 放宽匹配：只要是 zap.<Something>(<key>, ...) 且第一个参数为字符串字面量且等于 key，就视为匹配。
		if len(call.Args) == 0 {
			continue
		}

		bl, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			continue
		}

		var lit string
		if s, err := strconv.Unquote(bl.Value); err == nil {
			lit = s
		} else {
			lit = strings.Trim(bl.Value, "\"")
		}

		if lit == key {
			return i + 1
		}
	}

	return -1
}

// makeEllipsisAppend 构造 append([]zap.Field{zapStringArg}, expandedArg...) 表达式。
// 内层 append 调用设置 Ellipsis 以确保第二个参数被展开。
// 所有新建 AST 节点的位置都设置为 expandedArg.Pos(), 防止 go/printer 将函数间注释吸入表达式内部。
func makeEllipsisAppend(zapStringArg ast.Expr, expandedArg ast.Expr) *ast.CallExpr {
	pos := expandedArg.Pos()

	// 先递归设置 zapStringArg 的位置(它是新建的节点)
	setExprPos(zapStringArg, pos)

	headSlice := &ast.CompositeLit{
		Type: &ast.ArrayType{
			Lbrack: pos,
			Elt: &ast.SelectorExpr{
				X:   &ast.Ident{Name: zapIdent, NamePos: pos},
				Sel: &ast.Ident{Name: "Field", NamePos: pos},
			},
		},
		Elts:   []ast.Expr{zapStringArg},
		Lbrace: pos,
		Rbrace: pos,
	}

	return &ast.CallExpr{
		Fun:      &ast.Ident{Name: "append", NamePos: pos},
		Args:     []ast.Expr{headSlice, expandedArg},
		Lparen:   pos,
		Rparen:   expandedArg.End(),
		Ellipsis: pos, // 非零值表示第二个参数使用 ... 展开
	}
}

// setExprPos 递归设置 AST 表达式树中所有节点的位置。
// 用于确保新建的 AST 节点具有正确位置, 避免 go/printer 在格式化时将相邻注释错误地插入到表达式内部。
func setExprPos(e ast.Expr, pos token.Pos) {
	if e == nil {
		return
	}

	switch v := e.(type) {
	case *ast.CallExpr:
		if v.Fun != nil {
			setExprPos(v.Fun, pos)
		}

		v.Lparen = pos
		v.Rparen = pos

		for _, a := range v.Args {
			setExprPos(a, pos)
		}
	case *ast.BasicLit:
		v.ValuePos = pos
	case *ast.Ident:
		v.NamePos = pos
	case *ast.SelectorExpr:
		setExprPos(v.X, pos)
		v.Sel.NamePos = pos
	case *ast.CompositeLit:
		if v.Type != nil {
			setExprPos(v.Type, pos)
		}

		v.Lbrace = pos
		v.Rbrace = pos

		for _, el := range v.Elts {
			setExprPos(el, pos)
		}
	case *ast.ArrayType:
		v.Lbrack = pos

		if v.Elt != nil {
			setExprPos(v.Elt, pos)
		}
	}
}

// findEllipsisFieldCall 检查 ellipsis 展开参数是否已被 append([]zap.Field{zap.String(key, val)}, original...) 包裹。
// 如果匹配, 返回 append 调用、内部的 zap.String 调用、以及被包裹的原始参数(用于解包)。
// 如果不匹配, 返回 nil, nil, nil。
func findEllipsisFieldCall(expandedArg ast.Expr, key string) (*ast.CallExpr, *ast.CallExpr, ast.Expr) {
	appendCall, ok := expandedArg.(*ast.CallExpr)
	if !ok {
		return nil, nil, nil
	}

	appendIdent, ok := appendCall.Fun.(*ast.Ident)
	if !ok || appendIdent.Name != "append" {
		return nil, nil, nil
	}

	if len(appendCall.Args) != 2 {
		return nil, nil, nil
	}

	// 新模式: append([]zap.Field{zap.String(key, val)}, original...)
	// 第一个参数是 CompositeLit []zap.Field{...}, 包含一个 zap.String 调用
	compLit, ok := appendCall.Args[0].(*ast.CompositeLit)
	if !ok {
		return nil, nil, nil
	}

	if len(compLit.Elts) != 1 {
		return nil, nil, nil
	}

	zapCall, ok := compLit.Elts[0].(*ast.CallExpr)
	if !ok {
		return nil, nil, nil
	}

	funSel, ok := zapCall.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, nil, nil
	}

	id, ok := funSel.X.(*ast.Ident)
	if !ok || id.Name != zapIdent {
		return nil, nil, nil
	}

	// 放宽匹配: 只要是 zap.<Method>(<key>, ...) 且第一个参数为字符串字面量等于 key 即可
	if len(zapCall.Args) < 1 {
		return nil, nil, nil
	}

	bl, ok := zapCall.Args[0].(*ast.BasicLit)
	if !ok {
		return nil, nil, nil
	}

	var lit string
	if s, err := strconv.Unquote(bl.Value); err == nil {
		lit = s
	} else {
		lit = strings.Trim(bl.Value, "\"")
	}

	if lit != key {
		return nil, nil, nil
	}

	// 原始被展开的参数是第二个 append 参数
	return appendCall, zapCall, appendCall.Args[1]
}

// isZapChain 检查 expr 是否包含 zap.L() 的调用链, 例如 zap.L().With(...).Info 中的 receiver
func isZapChain(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.CallExpr:
		// 函数可能是 SelectorExpr (zap.L()) or another call
		if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if ident.Name == zapIdent && sel.Sel.Name == "L" {
					return true
				}
			}
		}

		return isZapChain(v.Fun)
	case *ast.SelectorExpr:
		return isZapChain(v.X)
	case *ast.Ident:
		return false
	default:
		return false
	}
}

// analyzeCallExpr 提取 handleCallExpr 与 verifyCallExpr 共享的检查与信息收集逻辑。
// 返回: isTarget(是否为目标 zap 日志调用), pos(左括号位置), rel(相对路径), funcName, pkgName, expected(期望注入字符串), foundIndex(在 ce.Args 中的真实索引, 未找到返回 -1)
func analyzeCallExpr(
	ce *ast.CallExpr,
	sel *ast.SelectorExpr,
	fSet *token.FileSet,
	file *ast.File,
	fns []fnRange,
	modulePath, baseDir string,
) (bool, token.Position, string, string, string, string, int) {
	method := sel.Sel.Name
	if !logMethods[method] {
		return false, token.Position{}, "", "", "", "", -1
	}

	if !isZapChain(sel.X) {
		return false, token.Position{}, "", "", "", "", -1
	}

	if len(ce.Args) == 0 {
		return false, token.Position{}, "", "", "", "", -1
	}

	pos := fSet.Position(ce.Lparen)

	// 使用 relPath 计算相对于仓库根的路径
	rel := relPath(pos.Filename, baseDir)

	callOffset := fSet.Position(ce.Pos()).Offset
	funcName := ""
	pkgName := file.Name.Name

	for _, fr := range fns {
		if callOffset >= fr.start && callOffset <= fr.end {
			funcName = fr.name
			pkgName = fr.pkg

			break
		}
	}

	expected := buildInjectedValue(rel, pos, funcName, pkgName, modulePath)

	foundIndex := findExistingFieldIndex(ce, *fieldFlg)

	return true, pos, rel, funcName, pkgName, expected, foundIndex
}

// correctLineNumbers 对 printer 输出进行二次修正:
// 重新解析输出文本, 用输出中的实际行号覆盖第一遍注入时使用的原始行号。
// 这样即使 go/printer 重排了某些代码行(例如多行 CompositeLit),
// 注入的 "file:line" 值也能与最终文件中的实际行号一致。
func correctLineNumbers(output string, path string, modulePath string, baseDir string) string {
	fSet2 := token.NewFileSet()

	file2, err := parser.ParseFile(fSet2, path, output, parser.ParseComments)
	if err != nil {
		return output
	}

	if !hasZapImport(file2) {
		return output
	}

	fns := collectFuncRanges(file2, fSet2)

	type lineEdit struct {
		offset int
		length int
		newVal string
	}

	var edits []lineEdit

	ast.Inspect(file2, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		isTarget, _, _, _, _, expected2, _ := analyzeCallExpr(ce, sel, fSet2, file2, fns, modulePath, baseDir)
		if !isTarget {
			return true
		}

		// 查找注入字段的值 BasicLit
		var bl *ast.BasicLit

		if ce.Ellipsis.IsValid() {
			lastIdx := len(ce.Args) - 1
			_, zapCall, _ := findEllipsisFieldCall(ce.Args[lastIdx], *fieldFlg)
			if zapCall != nil && len(zapCall.Args) >= 2 {
				if b, ok := zapCall.Args[1].(*ast.BasicLit); ok {
					bl = b
				}
			}
		} else {
			idx := findExistingFieldIndex(ce, *fieldFlg)
			if idx >= 0 {
				call, ok := ce.Args[idx].(*ast.CallExpr)
				if ok && len(call.Args) >= 2 {
					if b, ok := call.Args[1].(*ast.BasicLit); ok {
						bl = b
					}
				}
			}
		}

		if bl == nil {
			return true
		}

		actual, err := strconv.Unquote(bl.Value)
		if err != nil {
			return true
		}

		if actual != expected2 {
			newQuoted := strconv.Quote(expected2)
			offset := fSet2.Position(bl.ValuePos).Offset

			edits = append(edits, lineEdit{
				offset: offset,
				length: len(bl.Value),
				newVal: newQuoted,
			})
		}

		return true
	})

	if len(edits) == 0 {
		return output
	}

	// 从文件尾部向头部应用替换, 保证前面的偏移量不被后面的替换影响
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].offset > edits[j].offset
	})

	result := []byte(output)

	for _, e := range edits {
		tail := make([]byte, len(result[e.offset+e.length:]))
		copy(tail, result[e.offset+e.length:])
		result = append(result[:e.offset], append([]byte(e.newVal), tail...)...)
	}

	return string(result)
}
