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
		return handleDeleteField(ce, fSet)
	}

	// 使用 analyzeCallExpr 收集共享信息
	isTarget, pos, _, _, _, expected, foundIndex := analyzeCallExpr(ce, sel, fSet, file, fns, modulePath, baseDir)
	if !isTarget {
		return false, 0
	}

	// ellipsis 路径: 使用 append([]zap.Field{zap.String("fl", "...")}, expandedArg...) 包裹
	if ce.Ellipsis.IsValid() {
		return handleEllipsisInjection(ce, expected, pos)
	}

	// 非 ellipsis 路径: 直接插入或更新参数
	return handleNonEllipsisInsert(ce, expected, pos, foundIndex)
}

// handleDeleteField 处理删除字段逻辑, 返回是否修改以及修改所在的行号
func handleDeleteField(ce *ast.CallExpr, fSet *token.FileSet) (bool, int) {
	deleted := false

	if ce.Ellipsis.IsValid() {
		// ellipsis 调用: 检查展开参数是否被 append([]zap.Field{zap.String(delKey, ...)}, x...) 包裹, 解包还原
		lastIdx := len(ce.Args) - 1
		if _, _, origArg := findEllipsisFieldCall(ce.Args[lastIdx], *delFlg); origArg != nil {
			ce.Args[lastIdx] = origArg
			deleted = true
		}
	} else {
		if idxDel := findExistingFieldIndex(ce, *delFlg); idxDel >= 0 && idxDel < len(ce.Args) {
			ce.Args = append(ce.Args[:idxDel], ce.Args[idxDel+1:]...)
			deleted = true
		}
	}

	if deleted {
		pos := fSet.Position(ce.Lparen)
		return true, pos.Line
	}

	return false, 0
}

// handleEllipsisInjection 处理 ellipsis 场景的字段注入, 返回是否修改以及修改所在的行号
func handleEllipsisInjection(ce *ast.CallExpr, expected string, pos token.Position) (bool, int) {
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

// handleNonEllipsisInsert 处理非 ellipsis 场景的字段插入或更新, 返回是否修改以及修改所在的行号
func handleNonEllipsisInsert(ce *ast.CallExpr, expected string, pos token.Position, foundIndex int) (bool, int) {
	newArg := makeZapStringArg(expected)

	// 设置新节点位置, 防止 go/printer 将相邻注释吸入参数内部
	setExprPos(newArg, ce.Lparen)

	if foundIndex >= 0 {
		ce.Args[foundIndex] = newArg
		return true, pos.Line
	}

	// 计算插入索引: position 基于 field 参数列表(跳过第一个 msg 参数)
	// position=0 表示插入到第一个 field 之前(即 msg 之后), 默认(-1)等同于 0
	insertArgs(ce, newArg)

	// 如果要求按字母排序 zap 字段, 则对参数列表重新排序
	if *sortFlg {
		sortZapFields(ce)
	}

	return true, pos.Line
}

// insertArgs 将 newArg 插入到 ce.Args 中由 positionFlg 决定的位置
func insertArgs(ce *ast.CallExpr, newArg ast.Expr) {
	old := ce.Args

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
		if matchesZapFieldKey(a, key) {
			return i + 1
		}
	}

	return -1
}

// matchesZapFieldKey 判断参数表达式是否为 zap.<Method>(key, ...) 调用且第一个参数为字符串字面量等于 key
func matchesZapFieldKey(a ast.Expr, key string) bool {
	call, ok := a.(*ast.CallExpr)
	if !ok {
		return false
	}

	funSel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	id, ok := funSel.X.(*ast.Ident)
	if !ok || id.Name != zapIdent {
		return false
	}

	// 放宽匹配: 只要是 zap.<Something>(<key>, ...) 且第一个参数为字符串字面量且等于 key, 就视为匹配
	if len(call.Args) == 0 {
		return false
	}

	lit := parseLitKey(call.Args[0])

	return lit == key
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
		setCallExprPos(v, pos)
	case *ast.BasicLit:
		v.ValuePos = pos
	case *ast.Ident:
		v.NamePos = pos
	case *ast.SelectorExpr:
		setExprPos(v.X, pos)
		v.Sel.NamePos = pos
	case *ast.CompositeLit:
		setCompositeLitPos(v, pos)
	case *ast.ArrayType:
		setArrayTypePos(v, pos)
	}
}

// setCallExprPos 设置 CallExpr 节点及其子节点的位置
func setCallExprPos(v *ast.CallExpr, pos token.Pos) {
	if v.Fun != nil {
		setExprPos(v.Fun, pos)
	}

	v.Lparen = pos
	v.Rparen = pos

	for _, a := range v.Args {
		setExprPos(a, pos)
	}
}

// setCompositeLitPos 设置 CompositeLit 节点及其子节点的位置
func setCompositeLitPos(v *ast.CompositeLit, pos token.Pos) {
	if v.Type != nil {
		setExprPos(v.Type, pos)
	}

	v.Lbrace = pos
	v.Rbrace = pos

	for _, el := range v.Elts {
		setExprPos(el, pos)
	}
}

// setArrayTypePos 设置 ArrayType 节点及其子节点的位置
func setArrayTypePos(v *ast.ArrayType, pos token.Pos) {
	v.Lbrack = pos

	if v.Elt != nil {
		setExprPos(v.Elt, pos)
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

	edits := collectLineEdits(file2, fSet2, fns, modulePath, baseDir)

	if len(edits) == 0 {
		return output
	}

	return applyLineEdits(output, edits)
}

// lineEdit 描述一次行号修正替换
type lineEdit struct {
	offset int
	length int
	newVal string
}

// collectLineEdits 遍历 AST 收集所有需要修正行号的编辑项
func collectLineEdits(file2 *ast.File, fSet2 *token.FileSet, fns []fnRange, modulePath, baseDir string) []lineEdit {
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

		bl := findInjectedFieldLit(ce)
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

	return edits
}

// findInjectedFieldLit 在调用表达式中查找注入字段的值 BasicLit
func findInjectedFieldLit(ce *ast.CallExpr) *ast.BasicLit {
	if ce.Ellipsis.IsValid() {
		lastIdx := len(ce.Args) - 1
		_, zapCall, _ := findEllipsisFieldCall(ce.Args[lastIdx], *fieldFlg)

		if zapCall != nil && len(zapCall.Args) >= 2 {
			if b, ok := zapCall.Args[1].(*ast.BasicLit); ok {
				return b
			}
		}

		return nil
	}

	idx := findExistingFieldIndex(ce, *fieldFlg)
	if idx < 0 {
		return nil
	}

	call, ok := ce.Args[idx].(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return nil
	}

	if b, ok := call.Args[1].(*ast.BasicLit); ok {
		return b
	}

	return nil
}

// applyLineEdits 从文件尾部向头部应用替换, 保证前面的偏移量不被后面的替换影响
func applyLineEdits(output string, edits []lineEdit) string {
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
