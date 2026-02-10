//
// FilePath    : zap-smap\utils.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 工具函数
//

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jiaopengzi/go-utils"
)

// relPath 将 path 转换为相对于 baseDir 的标准化路径 (使用 '/' 作为分隔符)
func relPath(path, baseDir string) string {
	filename := path
	if !filepath.IsAbs(filename) {
		if abs, err := filepath.Abs(filename); err == nil {
			filename = abs
		}
	}

	if baseDir != "" {
		base := baseDir
		if !filepath.IsAbs(base) {
			if absBase, err := filepath.Abs(base); err == nil {
				base = absBase
			}
		}

		if rel, err := filepath.Rel(base, filename); err == nil {
			rel = filepath.Clean(rel)
			rel = filepath.ToSlash(rel)
			rel = strings.TrimPrefix(rel, "./")

			return rel
		}
	}

	// 兜底返回规范化后的输入路径
	p := filepath.Clean(filename)
	p = filepath.ToSlash(p)

	return p
}

// buildInjectedValue 构造注入的字符串值
func buildInjectedValue(rel string, pos token.Position, funcName, pkgName, modulePath string) string {
	// 规范路径为 '/' 分隔
	rel = filepath.ToSlash(rel)

	v := fmt.Sprintf("%s:%d", rel, pos.Line)

	if *funcFlg && funcName != "" {
		var funcFull string

		if modulePath != "" {
			dirRel := pathpkg.Dir(rel)

			importPath := modulePath

			if dirRel != "." && dirRel != "" {
				importPath = pathpkg.Join(modulePath, dirRel)
			}

			funcFull = fmt.Sprintf("%s.%s", importPath, funcName)
		} else {
			funcFull = fmt.Sprintf("%s.%s", pkgName, funcName)
		}

		v = fmt.Sprintf("%s | %s", v, funcFull)
	}

	return v
}

// hasZapImport 判断 ast.File 是否导入了 go.uber.org/zap
func hasZapImport(file *ast.File) bool {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, "\"") == "go.uber.org/zap" {
			return true
		}
	}

	return false
}

// collectFuncLitRanges 提取文件中所有匿名函数的范围信息并返回
func collectFuncLitRanges(file *ast.File, fSet *token.FileSet) []fnRange {
	var lits []fnRange

	ast.Inspect(file, func(n ast.Node) bool {
		if fl, ok := n.(*ast.FuncLit); ok {
			start := fSet.Position(fl.Pos()).Offset
			end := fSet.Position(fl.End()).Offset
			lits = append(lits, fnRange{start: start, end: end, pkg: file.Name.Name})
		}

		return true
	})

	if len(lits) == 0 {
		return nil
	}

	sort.Slice(lits, func(i, j int) bool { return lits[i].start < lits[j].start })

	return lits
}

// collectFuncRanges 收集文件中所有函数的字节范围和名称
func collectFuncRanges(file *ast.File, fSet *token.FileSet) []fnRange {
	var fns []fnRange

	// 遍历所有声明, 收集函数信息
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name == nil {
			continue
		}

		start := fSet.Position(fd.Pos()).Offset
		end := fSet.Position(fd.End()).Offset
		fName := buildFuncName(fd)
		pkg := file.Name.Name

		fns = append(fns, fnRange{start: start, end: end, name: fName, pkg: pkg})
	}

	// 收集文件内的匿名函数 (FuncLit)
	lits := collectFuncLitRanges(file, fSet)

	if len(lits) == 0 {
		return fns
	}

	// 在已知的函数范围中为每个匿名函数寻找最合适的父函数(最内层匹配)
	fns = matchAnonFuncs(fns, lits)

	return fns
}

// buildFuncName 从 FuncDecl 构建完整的函数名(包含接收器类型名)
func buildFuncName(fd *ast.FuncDecl) string {
	fName := fd.Name.Name

	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fName
	}

	return receiverTypeName(fd.Recv.List[0].Type) + "." + fName
}

// receiverTypeName 从接收器类型中提取类型名
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}

	return ""
}

// matchAnonFuncs 为每个匿名函数在已知函数范围中寻找最合适的父函数(最内层匹配),
// 并将匿名函数追加到函数列表中
func matchAnonFuncs(fns []fnRange, lits []fnRange) []fnRange {
	for _, lr := range lits {
		bestIdx := findBestParent(fns, lr)

		name := "<anonymous>"
		if bestIdx >= 0 {
			name = fns[bestIdx].name + ".<anonymous>"
		}

		fns = append(fns, fnRange{start: lr.start, end: lr.end, name: name, pkg: lr.pkg})
	}

	return fns
}

// findBestParent 在函数列表中找到包含 lr 范围的最内层函数索引, 未找到返回 -1
func findBestParent(fns []fnRange, lr fnRange) int {
	bestIdx := -1
	bestStart := -1

	for i, fr := range fns {
		if lr.start >= fr.start && lr.end <= fr.end {
			if bestIdx == -1 || fr.start > bestStart {
				bestIdx = i
				bestStart = fr.start
			}
		}
	}

	return bestIdx
}

// getZapFieldKey 从 zap 字段调用 (例如 zap.String("key", "val")) 中提取 key
func getZapFieldKey(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}

	bl, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return ""
	}

	return unquoteLiteral(bl.Value)
}

// unquoteLiteral 将字符串字面量值解引号, 如果 Unquote 失败则去除两侧双引号
func unquoteLiteral(raw string) string {
	if s, err := strconv.Unquote(raw); err == nil {
		return s
	}

	return strings.Trim(raw, "\"")
}

// extractZapFieldKV 从单个 zap 字段调用表达式中提取 "key=value" 字符串。
// 如果不是有效的 zap 字段调用, 返回空字符串
func extractZapFieldKV(a ast.Expr) string {
	call, ok := a.(*ast.CallExpr)
	if !ok {
		return ""
	}

	funSel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	id, ok := funSel.X.(*ast.Ident)
	if !ok || id.Name != zapIdent {
		return ""
	}

	if len(call.Args) == 0 {
		return ""
	}

	key := parseLitKey(call.Args[0])
	if key == "" {
		return ""
	}

	val := parseLitVal(call.Args)

	return fmt.Sprintf("%s=%s", key, val)
}

// parseLitKey 从 AST 表达式中解析字符串字面量的 key 值
func parseLitKey(expr ast.Expr) string {
	bl, ok := expr.(*ast.BasicLit)
	if !ok {
		return ""
	}

	return unquoteLiteral(bl.Value)
}

// parseLitVal 从 zap 字段参数列表中解析 value 值
func parseLitVal(args []ast.Expr) string {
	if len(args) <= 1 {
		return "<missing>"
	}

	bl, ok := args[1].(*ast.BasicLit)
	if !ok {
		return "<non-literal>"
	}

	return unquoteLiteral(bl.Value)
}

// collectExistingFields 遍历 ce.Args[1:], 收集所有 zap 字段调用的 "key=value" 字符串列表
func collectExistingFields(ce *ast.CallExpr) []string {
	var fields []string

	for _, a := range ce.Args[1:] {
		if kv := extractZapFieldKV(a); kv != "" {
			fields = append(fields, kv)
		}
	}

	return fields
}

// isZapFieldCall 判断 expr 是否为 zap.String/Any/Uint64 的调用, 并返回对应的 CallExpr
func isZapFieldCall(expr ast.Expr) (*ast.CallExpr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}

	funSel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}

	id, ok := funSel.X.(*ast.Ident)
	if !ok || id.Name != zapIdent {
		return nil, false
	}

	name := funSel.Sel.Name
	if name != zapMethodString && name != zapMethodAny && name != zapMethodUint64 {
		return nil, false
	}

	return call, true
}

// normalizeBaseDir 将 dir 参数标准化为目录路径(如果传入文件则返回其所在目录)
func normalizeBaseDir(dir string) (string, error) {
	baseDir := dir
	if fi, err := os.Stat(baseDir); err == nil {
		if !fi.IsDir() {
			// 如果传入的是文件路径, 使用工作目录作为仓库根路径,
			// 以便生成相对于仓库根的文件路径 (例如 "cron/task_coupon_status.go")
			wd, err := os.Getwd()
			if err != nil {
				return "", fmt.Errorf("os.Getwd failed: %w", err)
			}

			return wd, nil
		}
	}

	return baseDir, nil
}

// readModulePath 尝试读取 baseDir 下的 go.mod, 返回 module path(若失败返回空串)
func readModulePath(baseDir string) string {
	goModPath := filepath.Join(baseDir, "go.mod")
	if b, err := utils.ReadFile(goModPath); err == nil {
		for line := range strings.SplitSeq(string(b), "\n") {
			line = strings.TrimSpace(line)
			if after, ok := strings.CutPrefix(line, "module "); ok {
				return strings.TrimSpace(after)
			}
		}
	}

	return ""
}
