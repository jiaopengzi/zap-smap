//
// FilePath    : zap-smap\verify.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 在不修改文件的情况下校验注入字段
//

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"

	"github.com/jiaopengzi/go-utils"
)

// verifyFile 在不修改文件的情况下校验每个 zap 日志调用的注入字段是否存在且值是否正确
func verifyFile(path string, fSet *token.FileSet, modulePath string, baseDir string) (int, int, int, []string, error) {
	// 读取文件内容
	src, err := utils.ReadFile(path)
	if err != nil {
		return 0, 0, 0, nil, err
	}

	// 解析文件为 AST
	file, err := parser.ParseFile(fSet, path, src, parser.ParseComments)
	if err != nil {
		// 解析失败时至少记录警告, 避免静默跳过
		fmt.Fprintf(os.Stderr, "warn: parse %s failed: %v\n", path, err)
		return 0, 0, 0, nil, nil
	}

	// 判断是否包含 zap 导入
	// 如果没有导入 go.uber.org/zap, 则无需继续处理, 提前返回
	if !hasZapImport(file) {
		return 0, 0, 0, nil, nil
	}

	// 初始化统计计数
	total := 0
	missing := 0
	mismatch := 0

	var issues []string

	// 收集文件中每个函数的范围信息, 用于后续定位调用处所属的函数(以便构造完整函数路径)
	fns := collectFuncRanges(file, fSet)

	// 遍历 AST 节点:定位所有函数调用并委托给 verifyCallExpr 完成单次调用的校验
	// verifyCallExpr 会返回是否为目标日志调用以及是否存在问题类型(缺失或不匹配)
	ast.Inspect(file, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// 对单个调用进行校验
		shouldCount, issue, isMissing, isMismatch := verifyCallExpr(ce, sel, fSet, file, fns, modulePath, baseDir)
		if !shouldCount {
			return true
		}

		// 统计目标日志调用总数
		total++

		// 如果 verifyCallExpr 返回了 issue, 则记录并根据问题类型更新相应计数器
		if issue != "" {
			issues = append(issues, issue)

			if isMissing {
				missing++
			}

			if isMismatch {
				mismatch++
			}
		}

		return true
	})

	return total, missing, mismatch, issues, nil
}

// verifyAndHandleSingleFile 对单个文件执行 verify 并处理结果
func verifyAndHandleSingleFile(path string, fSet *token.FileSet, modulePath, baseDir string) error {
	total, _, _, _, err := reportVerifyForPath(path, fSet, modulePath, baseDir)
	if err != nil {
		return err
	}

	// 当前文件无注入点或者无问题: nothing more to do
	if total == 0 {
		return nil
	}

	return nil
}

// reportVerifyForPath 运行 verifyFile 并打印问题（如果有），返回统计数据
func reportVerifyForPath(path string, fSet *token.FileSet, modulePath, baseDir string) (int, int, int, []string, error) {
	total, missing, mismatch, issues, err := verifyFile(path, fSet, modulePath, baseDir)
	if err != nil {
		return 0, 0, 0, nil, err
	}

	if total == 0 {
		return total, missing, mismatch, issues, nil
	}

	if len(issues) > 0 {
		rel := relPath(path, baseDir)
		fmt.Printf("[VERIFY] %s: total=%d missing=%d mismatch=%d\n", rel, total, missing, mismatch)

		for _, it := range issues {
			fmt.Println(it)
		}
	}

	return total, missing, mismatch, issues, nil
}

// printVerifySummary 打印汇总报告
func printVerifySummary(totalAll, missingAll, mismatchAll int) {
	fmt.Printf("\n===== VERIFY SUMMARY =====\n")
	fmt.Printf("total calls: %d\nmissing: %d\nmismatch: %d\n", totalAll, missingAll, mismatchAll)

	if missingAll == 0 && mismatchAll == 0 {
		fmt.Println("All injections look correct.")
	}
}

// verifyCallExpr 验证单次 zap 日志调用是否包含正确的注入字段
// 返回: shouldCount(是否为目标日志调用需要计入统计), issue(若不为空则为问题描述), isMissing, isMismatch
func verifyCallExpr(
	ce *ast.CallExpr,
	sel *ast.SelectorExpr,
	fSet *token.FileSet,
	file *ast.File,
	fns []fnRange,
	modulePath, baseDir string,
) (bool, string, bool, bool) {
	isTarget, pos, rel, _, _, expected, foundIndex := analyzeCallExpr(ce, sel, fSet, file, fns, modulePath, baseDir)
	if !isTarget {
		return false, "", false, false
	}

	method := sel.Sel.Name

	// ellipsis 路径: 检查 append 包裹内部的注入字段
	if ce.Ellipsis.IsValid() {
		lastIdx := len(ce.Args) - 1
		expandedArg := ce.Args[lastIdx]

		_, zapCall, _ := findEllipsisFieldCall(expandedArg, *fieldFlg)

		if zapCall == nil {
			// 缺失: 展开参数未被 append([]zap.Field{zap.String("fl", "...")}, x...) 包裹
			return true, fmt.Sprintf("%s:%d: zap.%s missing field '%s' (ellipsis call), expected='%s'", rel, pos.Line, method, *fieldFlg, expected), true, false
		}

		// 检查值是否匹配
		if len(zapCall.Args) < 2 {
			return true, fmt.Sprintf("%s:%d: zap.%s field '%s' has insufficient args in append wrapper", rel, pos.Line, method, *fieldFlg), false, false
		}

		bl, ok := zapCall.Args[1].(*ast.BasicLit)
		if !ok {
			return true, fmt.Sprintf("%s:%d: zap.%s field '%s' value is not a string literal", rel, pos.Line, method, *fieldFlg), false, false
		}

		var actual string
		if s, err := strconv.Unquote(bl.Value); err == nil {
			actual = s
		} else {
			actual = strings.Trim(bl.Value, "\"")
		}

		if actual != expected {
			return true, fmt.Sprintf("%s:%d: zap.%s field '%s' mismatch actual='%s' expected='%s'", rel, pos.Line, method, *fieldFlg, actual, expected), false, true
		}

		return true, "", false, false
	}

	// 非 ellipsis 路径: 原有逻辑
	// 收集现有字段列表用于更友好的错误提示
	var existingFields []string

	for _, a := range ce.Args[1:] {
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

		if len(call.Args) == 0 {
			continue
		}

		// 尝试解析 key/value
		var key string

		if bl, ok := call.Args[0].(*ast.BasicLit); ok {
			if s, err := strconv.Unquote(bl.Value); err == nil {
				key = s
			} else {
				key = strings.Trim(bl.Value, "\"")
			}
		}

		var val string

		if len(call.Args) > 1 {
			if bl2, ok := call.Args[1].(*ast.BasicLit); ok {
				if s, err := strconv.Unquote(bl2.Value); err == nil {
					val = s
				} else {
					val = strings.Trim(bl2.Value, "\"")
				}
			} else {
				val = "<non-literal>"
			}
		} else {
			val = "<missing>"
		}

		if key != "" {
			existingFields = append(existingFields, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// 非 ellipsis: method 已经在上方声明，直接使用

	if foundIndex < 0 {
		existStr := strings.Join(existingFields, ", ")
		return true, fmt.Sprintf("%s:%d: zap.%s missing field '%s', expected='%s', existing fields: [%s]", rel, pos.Line, method, *fieldFlg, expected, existStr), true, false
	}

	issue, isMismatch, actual, existing := verifyExistingField(ce, foundIndex, expected, pos, baseDir)
	if issue != "" {
		existStr := strings.Join(existing, ", ")

		if isMismatch {
			return true, fmt.Sprintf("%s:%d: zap.%s field '%s' mismatch actual='%s' expected='%s', existing fields: [%s]", rel, pos.Line, method, *fieldFlg, actual, expected, existStr), false, true
		}

		// 其他类型的问题（非值不匹配）
		// 从 issue 中提取简短描述
		parts := strings.SplitN(issue, ": ", 2)
		detail := issue

		if len(parts) == 2 {
			detail = parts[1]
		}

		return true, fmt.Sprintf("%s:%d: zap.%s %s, existing fields: [%s]", rel, pos.Line, method, detail, existStr), false, false
	}

	return true, "", false, false
}

// verifyExistingField 检查已存在的字段表达式是否为 zap.String(key, value) 形式, 且 key 为 fieldFlg,
// value 为期望的字符串值 expected。
//
// 参数:
//   - ce: 包含该字段参数的调用表达式(外层日志调用的参数列表)
//   - foundIndex: 在 ce.Args 中该字段参数的索引
//   - expected: 期望的字符串值(由 buildInjectedValue 构造)
//   - pos: 调用位置(用于构造文件:行号的错误信息)
//
// 返回:
//   - issue: 若非空表示存在问题, 包含文件与行号及错误描述; 为空表示校验通过
//   - isMismatch: 如果问题类型为值不匹配(actual != expected)则为 true, 其他错误类型返回 false
func verifyExistingField(ce *ast.CallExpr, foundIndex int, expected string, pos token.Position, baseDir string) (string, bool, string, []string) {
	// 先收集现有字段列表
	var existing []string

	for _, a := range ce.Args[1:] {
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

		if len(call.Args) == 0 {
			continue
		}

		var key string

		if bl, ok := call.Args[0].(*ast.BasicLit); ok {
			if s, err := strconv.Unquote(bl.Value); err == nil {
				key = s
			} else {
				key = strings.Trim(bl.Value, "\"")
			}
		}

		var val string

		if len(call.Args) > 1 {
			if bl2, ok := call.Args[1].(*ast.BasicLit); ok {
				if s, err := strconv.Unquote(bl2.Value); err == nil {
					val = s
				} else {
					val = strings.Trim(bl2.Value, "\"")
				}
			} else {
				val = "<non-literal>"
			}
		} else {
			val = "<missing>"
		}

		if key != "" {
			existing = append(existing, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// 1) 确认该参数是一个调用表达式 (例如: zap.String(...))
	call, ok := ce.Args[foundIndex].(*ast.CallExpr)
	if !ok {
		rel := relPath(pos.Filename, baseDir)
		return fmt.Sprintf("%s:%d: field arg not a call expression", rel, pos.Line), true, "", existing
	}

	// 2) 确认调用的函数是一个 SelectorExpr (例如 zap.String)
	funSel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		rel := relPath(pos.Filename, baseDir)
		return fmt.Sprintf("%s:%d: unexpected expression for field arg", rel, pos.Line), true, "", existing
	}

	// 3) 确认接收者为 `zap` 且方法名为 `String`
	id, ok := funSel.X.(*ast.Ident)
	if !ok || id.Name != zapIdent || funSel.Sel.Name != zapMethodString {
		rel := relPath(pos.Filename, baseDir)
		return fmt.Sprintf("%s:%d: expected zap.String call for field", rel, pos.Line), true, "", existing
	}

	// 4) zap.String 至少应有两个参数 (key, value)
	if len(call.Args) <= 1 {
		rel := relPath(pos.Filename, baseDir)
		return fmt.Sprintf("%s:%d: zap.String has insufficient args", rel, pos.Line), true, "", existing
	}

	// 5) 第二个参数应为字符串字面量
	bl, ok := call.Args[1].(*ast.BasicLit)
	if !ok {
		rel := relPath(pos.Filename, baseDir)
		return fmt.Sprintf("%s:%d: mismatched type for field, expected basic literal", rel, pos.Line), true, "", existing
	}

	// 6) 解析字面量为字符串并与期望值比较
	var actual string
	if s, err := strconv.Unquote(bl.Value); err == nil {
		actual = s
	} else {
		actual = strings.Trim(bl.Value, "\"")
	}

	if actual != expected {
		rel := relPath(pos.Filename, baseDir)
		return fmt.Sprintf("%s:%d: mismatch actual='%s' expected='%s'", rel, pos.Line, actual, expected), true, actual, existing
	}

	// 校验通过
	return "", false, actual, existing
}
