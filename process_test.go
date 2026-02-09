//
// FilePath    : zap-smap\process_test.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : ellipsis (fields...) 场景单测
//

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain_EllipsisFields_InjectsWithAppendPrepend 测试 fields... 展开调用使用 append([]zap.Field{...}, fields...) 注入到切片第一位
func TestMain_EllipsisFields_InjectsWithAppendPrepend(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "ellipsis.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", fields...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "ellipsis.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 应包含 append([]zap.Field{zap.String("fl", "...")}, fields...)...
	if !strings.Contains(s, `append([]zap.Field{zap.String("fl"`) {
		t.Fatalf("expected append([]zap.Field{zap.String(\"fl\"...)} wrapper, got:\n%s", s)
	}

	// 确认 fl 字段在 fields 之前 (在切片第一位)
	idxFl := strings.Index(s, `zap.String("fl"`)
	idxFields := strings.Index(s, "fields...")
	if idxFl < 0 || idxFields < 0 {
		t.Fatalf("expected both fl and fields... present, got:\n%s", s)
	}
	if idxFl > idxFields {
		t.Fatalf("expected fl before fields... (fl at first position), got:\n%s", s)
	}
}

// TestMain_EllipsisFields_Idempotent 测试 fields... 场景的幂等性: 二次运行只更新值不重复包裹
func TestMain_EllipsisFields_Idempotent(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "idem.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", fields...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	// 第一次运行
	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "idem.go")
	b1, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file after first run: %v", err)
	}

	first := string(b1)
	if !strings.Contains(first, `append([]zap.Field{zap.String("fl"`) {
		t.Fatalf("first run should inject append wrapper, got:\n%s", first)
	}

	// 第二次运行
	resetGlobals()
	resetNewFlags()
	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	b2, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file after second run: %v", err)
	}

	second := string(b2)

	// 不应出现嵌套 append
	if strings.Contains(second, "append([]zap.Field{zap.String(\"fl\"") &&
		strings.Count(second, "append(") > strings.Count(first, "append(") {
		t.Fatalf("second run should not add nested append, got:\n%s", second)
	}
}

// TestMain_EllipsisFields_NormalCallsStillWork 测试混合文件中普通调用不受 ellipsis 逻辑影响
func TestMain_EllipsisFields_NormalCallsStillWork(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "mixed.go", `package sample

import "go.uber.org/zap"

func Foo() {
	zap.L().Info("normal call")
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Error("with fields", fields...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "mixed.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 普通调用应直接注入 zap.String("fl", "...")
	if !strings.Contains(s, `zap.L().Info("normal call", zap.String("fl"`) {
		t.Fatalf("normal call should have direct injection, got:\n%s", s)
	}

	// ellipsis 调用应使用 append 包裹
	if !strings.Contains(s, `append([]zap.Field{zap.String("fl"`) {
		t.Fatalf("ellipsis call should have append wrapper, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_DelFlag 测试 -del 能正确解包 ellipsis 调用中的注入字段
func TestMain_EllipsisFields_DelFlag(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	// 先写入已注入的文件 (使用 append 包裹)
	writeFile(t, td, "del_el.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", append([]zap.Field{zap.String("fl", "del_el.go:7")}, fields...)...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*delFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "del_el.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 删除后不应包含 append 包裹和 fl 字段
	if strings.Contains(s, `zap.String("fl"`) {
		t.Fatalf("expected fl field removed, got:\n%s", s)
	}

	// 应还原为 fields...
	if !strings.Contains(s, "fields...") {
		t.Fatalf("expected fields... restored after del, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_VerifyMode 测试 -verify 能正确校验 ellipsis 调用中的注入字段
func TestMain_EllipsisFields_VerifyMode(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "ver_el.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", append([]zap.Field{zap.String("fl", "ver_el.go:7")}, fields...)...)
}
`)

	*pathFlag = td
	*verifyFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	out := captureOutput(func() { main() })

	if !strings.Contains(out, "missing: 0") || !strings.Contains(out, "mismatch: 0") {
		t.Fatalf("expected verify with zero issues, got:\n%s", out)
	}
}

// TestMain_EllipsisFields_VerifyMode_Missing 测试 -verify 对未注入的 ellipsis 调用报告 missing
func TestMain_EllipsisFields_VerifyMode_Missing(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "ver_miss.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", fields...)
}
`)

	*pathFlag = td
	*verifyFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	out := captureOutput(func() { main() })

	if !strings.Contains(out, "missing") {
		t.Fatalf("expected verify to report missing for non-injected ellipsis call, got:\n%s", out)
	}
}

// TestMain_EllipsisFields_VerifyMode_Mismatch 测试 -verify 对 ellipsis 调用中值不匹配报告 mismatch
func TestMain_EllipsisFields_VerifyMode_Mismatch(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "ver_mis.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", append([]zap.Field{zap.String("fl", "wrong-value")}, fields...)...)
}
`)

	*pathFlag = td
	*verifyFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	out := captureOutput(func() { main() })

	if !strings.Contains(out, "mismatch") {
		t.Fatalf("expected verify to report mismatch for ellipsis call, got:\n%s", out)
	}
}

// TestMain_EllipsisFields_WithFuncFlag 测试 ellipsis 调用在 -with-func 模式下值包含函数名
func TestMain_EllipsisFields_WithFuncFlag(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "func_el.go", `package sample

import "go.uber.org/zap"

func MyHandler() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", fields...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	*funcFlg = true
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "func_el.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 应包含函数名
	if !strings.Contains(s, "sample.MyHandler") {
		t.Fatalf("expected injected value to contain function name 'sample.MyHandler', got:\n%s", s)
	}

	// 仍应使用 append 包裹
	if !strings.Contains(s, `append([]zap.Field{zap.String("fl"`) {
		t.Fatalf("expected append wrapper for ellipsis call with -with-func, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_InlineSlice 测试内联切片 []zap.Field{...}... 展开场景
func TestMain_EllipsisFields_InlineSlice(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "inline.go", `package sample

import "go.uber.org/zap"

func Foo() {
	zap.L().Info("inline", []zap.Field{
		zap.String("key1", "val1"),
	}...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "inline.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 内联切片展开也应使用 append 包裹
	if !strings.Contains(s, `append([]zap.Field{zap.String("fl"`) {
		t.Fatalf("expected append wrapper for inline slice ellipsis, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_FuncReturnValue 测试函数返回值展开 getFields()... 场景
func TestMain_EllipsisFields_FuncReturnValue(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "funcret.go", `package sample

import "go.uber.org/zap"

func getFields() []zap.Field {
	return []zap.Field{zap.String("source", "config")}
}

func Foo() {
	zap.L().Debug("from func", getFields()...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "funcret.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 应包含 append 包裹且 getFields() 在第二个参数
	if !strings.Contains(s, `append([]zap.Field{zap.String("fl"`) {
		t.Fatalf("expected append wrapper for func return call, got:\n%s", s)
	}
	if !strings.Contains(s, "getFields()") {
		t.Fatalf("expected getFields() preserved in call, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_MultipleCallsSameFile 测试同一文件中多个 ellipsis 调用各自被正确注入
func TestMain_EllipsisFields_MultipleCallsSameFile(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "multi_el.go", `package sample

import "go.uber.org/zap"

func Foo() {
	f1 := []zap.Field{zap.String("a", "1")}
	zap.L().Info("first", f1...)

	f2 := []zap.Field{zap.String("b", "2")}
	zap.L().Error("second", f2...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "multi_el.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 两个 ellipsis 调用都应被包裹
	count := strings.Count(s, `append([]zap.Field{zap.String("fl"`)
	if count != 2 {
		t.Fatalf("expected 2 append wrappers for 2 ellipsis calls, got %d:\n%s", count, s)
	}

	// 行号应不同
	if !strings.Contains(s, "multi_el.go:7") || !strings.Contains(s, "multi_el.go:10") {
		t.Fatalf("expected different line numbers for each call, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_DryRun 测试 dry-run 模式(不写回) 对 ellipsis 调用也能正确预览
func TestMain_EllipsisFields_DryRun(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "dryrun.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", fields...)
}
`)

	*pathFlag = td
	*writeFlg = false // dry-run
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	out := captureOutput(func() { main() })

	// 应输出 PATCH 预览
	if !strings.Contains(out, "[PATCH]") {
		t.Fatalf("expected PATCH output in dry-run, got:\n%s", out)
	}

	// 文件不应被修改
	p := filepath.Join(td, "dryrun.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)
	if strings.Contains(s, "append(") {
		t.Fatalf("file should NOT be modified in dry-run mode, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_AnonFunc 测试匿名函数中 ellipsis 调用的注入
func TestMain_EllipsisFields_AnonFunc(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "anon_el.go", `package sample

import "go.uber.org/zap"

func Foo() {
	handler := func() {
		fields := []zap.Field{zap.String("key", "val")}
		zap.L().Warn("inside anon", fields...)
	}
	handler()
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "anon_el.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 匿名函数中的 ellipsis 调用也应使用 append 包裹
	if !strings.Contains(s, `append([]zap.Field{zap.String("fl"`) {
		t.Fatalf("expected append wrapper for ellipsis call in anon func, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_DelThenInject 测试先删除再注入的流程
func TestMain_EllipsisFields_DelThenInject(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	// 文件已有旧字段 "old" 的 append 包裹
	writeFile(t, td, "rein.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", append([]zap.Field{zap.String("old", "rein.go:7")}, fields...)...)
}
`)

	// 步骤1: 删除旧字段 "old"
	*pathFlag = td
	*writeFlg = true
	*delFlg = "old"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "rein.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file after del: %v", err)
	}

	s := string(b)
	if strings.Contains(s, `zap.String("old"`) {
		t.Fatalf("expected old field removed, got:\n%s", s)
	}
	// 纯删除后 append 包裹也应被还原
	if strings.Contains(s, "append(") {
		t.Fatalf("expected append wrapper removed after del, got:\n%s", s)
	}
	if !strings.Contains(s, "fields...") {
		t.Fatalf("expected fields... restored after del, got:\n%s", s)
	}

	// 步骤2: 注入新字段 "fl"
	resetGlobals()
	resetNewFlags()
	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	b2, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file after inject: %v", err)
	}

	s2 := string(b2)
	if !strings.Contains(s2, `append([]zap.Field{zap.String("fl"`) {
		t.Fatalf("expected new fl field injected with append wrapper, got:\n%s", s2)
	}
	if !strings.Contains(s2, "fields...") {
		t.Fatalf("expected fields... present after re-inject, got:\n%s", s2)
	}
}

// TestMain_EllipsisFields_IdempotentValueUpdate 测试幂等场景中文件被移动后值会更新(非重复包裹)
func TestMain_EllipsisFields_IdempotentValueUpdate(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	// 模拟一个已注入但值不正确(例如文件名改变)的文件
	writeFile(t, td, "moved.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", append([]zap.Field{zap.String("fl", "old_name.go:7")}, fields...)...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "moved.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 值应被更新为新路径
	if !strings.Contains(s, "moved.go:7") {
		t.Fatalf("expected value updated to moved.go:7, got:\n%s", s)
	}
	// 旧值不应残留
	if strings.Contains(s, "old_name.go") {
		t.Fatalf("expected old value removed, got:\n%s", s)
	}
	// 不应有嵌套 append
	if strings.Count(s, "append(") != 1 {
		t.Fatalf("expected exactly 1 append call (no nesting), got:\n%s", s)
	}
}

// TestMain_EllipsisFields_AllLogMethods 测试所有 zap 日志方法(Debug/Info/Warn/Error/DPanic/Panic/Fatal) 的 ellipsis 场景
func TestMain_EllipsisFields_AllLogMethods(t *testing.T) {
	methods := []string{"Debug", "Info", "Warn", "Error", "DPanic", "Panic", "Fatal"}

	for _, method := range methods {
		method := method
		t.Run(method, func(t *testing.T) {
			resetGlobals()
			resetNewFlags()

			td := t.TempDir()
			content := `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().` + method + `("msg", fields...)
}
`
			fname := strings.ToLower(method) + "_el.go"
			writeFile(t, td, fname, content)

			*pathFlag = td
			*writeFlg = true
			*fieldFlg = "fl"
			os.Args = []string{"cmd"}

			_ = captureOutput(func() { main() })

			p := filepath.Join(td, fname)
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("read file: %v", err)
			}

			s := string(b)
			if !strings.Contains(s, `append([]zap.Field{zap.String("fl"`) {
				t.Fatalf("expected append wrapper for %s ellipsis call, got:\n%s", method, s)
			}
		})
	}
}

// TestMain_EllipsisFields_DelOldField_Unwraps 测试 -del 对 ellipsis 调用中旧字段的纯删除(解包还原)
// 输入: 文件中 ellipsis 调用已用 append([]zap.Field{zap.String("file:line", ...)}, fields...) 包裹
// 期望: -del "file:line" 解包 append, 还原为 fields... (纯删除, 不注入新字段)
func TestMain_EllipsisFields_DelOldField_Unwraps(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "del_slice.go", `package sample

import "go.uber.org/zap"

func Foo() {
	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Info("hello", append([]zap.Field{zap.String("file:line", "del_slice.go:7")}, fields...)...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*delFlg = "file:line"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "del_slice.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 旧字段 "file:line" 应被完全移除
	if strings.Contains(s, `"file:line"`) {
		t.Fatalf("expected 'file:line' removed, got:\n%s", s)
	}

	// 不应有 append 包裹 (纯删除, 不注入)
	if strings.Contains(s, "append(") {
		t.Fatalf("expected append wrapper removed (pure delete), got:\n%s", s)
	}

	// fields... 应被还原
	if !strings.Contains(s, "fields...") {
		t.Fatalf("expected fields... restored, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_DelOldField_Mixed 测试混合文件(ellipsis + 普通调用)中 -del 的纯删除行为
func TestMain_EllipsisFields_DelOldField_Mixed(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "del_mix.go", `package sample

import "go.uber.org/zap"

func Foo() {
	zap.L().Info("normal", zap.String("file:line", "del_mix.go:6"))

	fields := []zap.Field{zap.String("key", "val")}
	zap.L().Error("with fields", append([]zap.Field{zap.String("file:line", "del_mix.go:9")}, fields...)...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*delFlg = "file:line"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "del_mix.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 旧字段 "file:line" 应被完全移除
	if strings.Contains(s, `"file:line"`) {
		t.Fatalf("expected 'file:line' fully removed from mixed file, got:\n%s", s)
	}

	// 普通调用应恢复为无字段: zap.L().Info("normal")
	if !strings.Contains(s, `zap.L().Info("normal")`) {
		t.Fatalf("expected normal call to have field removed, got:\n%s", s)
	}

	// ellipsis 调用应还原为 fields... (无 append 包裹)
	if strings.Contains(s, "append(") {
		t.Fatalf("expected append wrapper removed from ellipsis call, got:\n%s", s)
	}
	if !strings.Contains(s, "fields...") {
		t.Fatalf("expected fields... restored, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_DelOldField_MultipleEllipsis 测试多个 ellipsis 调用中旧字段全部被纯删除
func TestMain_EllipsisFields_DelOldField_MultipleEllipsis(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "del_multi.go", `package sample

import "go.uber.org/zap"

func Foo() {
	f1 := []zap.Field{zap.String("a", "1")}
	zap.L().Info("first", append([]zap.Field{zap.String("file:line", "del_multi.go:7")}, f1...)...)

	f2 := []zap.Field{zap.String("b", "2")}
	zap.L().Error("second", append([]zap.Field{zap.String("file:line", "del_multi.go:10")}, f2...)...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*delFlg = "file:line"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "del_multi.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 旧字段 "file:line" 应被完全移除
	if strings.Contains(s, `"file:line"`) {
		t.Fatalf("expected all 'file:line' removed, got:\n%s", s)
	}

	// 两个 ellipsis 调用的 append 包裹都应被还原
	if strings.Contains(s, "append(") {
		t.Fatalf("expected all append wrappers removed, got:\n%s", s)
	}

	// f1... 和 f2... 应被还原
	if !strings.Contains(s, "f1...") || !strings.Contains(s, "f2...") {
		t.Fatalf("expected f1... and f2... restored, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_DelOldField_InlineSlice 测试内联切片展开调用中旧字段的纯删除
func TestMain_EllipsisFields_DelOldField_InlineSlice(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "del_inline.go", `package sample

import "go.uber.org/zap"

func Foo() {
	zap.L().Info("inline", append([]zap.Field{zap.String("file:line", "del_inline.go:6")}, []zap.Field{
		zap.String("key1", "val1"),
	}...)...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*delFlg = "file:line"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "del_inline.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 旧字段 "file:line" 应被完全移除
	if strings.Contains(s, `"file:line"`) {
		t.Fatalf("expected 'file:line' removed from inline slice call, got:\n%s", s)
	}

	// append 包裹应被还原
	if strings.Contains(s, "append(") {
		t.Fatalf("expected append wrapper removed, got:\n%s", s)
	}

	// 内联切片应保留
	if !strings.Contains(s, `zap.String("key1", "val1")`) {
		t.Fatalf("expected inline slice preserved, got:\n%s", s)
	}
}

// TestMain_EllipsisFields_DelOldField_FuncReturn 测试函数返回值展开调用中旧字段的纯删除
func TestMain_EllipsisFields_DelOldField_FuncReturn(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "del_fr.go", `package sample

import "go.uber.org/zap"

func getF() []zap.Field { return nil }

func Foo() {
	zap.L().Debug("from func", append([]zap.Field{zap.String("file:line", "del_fr.go:8")}, getF()...)...)
}
`)

	*pathFlag = td
	*writeFlg = true
	*delFlg = "file:line"
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "del_fr.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)

	// 旧字段移除
	if strings.Contains(s, `"file:line"`) {
		t.Fatalf("expected 'file:line' removed, got:\n%s", s)
	}

	// append 包裹应被还原
	if strings.Contains(s, "append(") {
		t.Fatalf("expected append wrapper removed, got:\n%s", s)
	}

	// getF()... 应被还原
	if !strings.Contains(s, "getF()...") {
		t.Fatalf("expected getF()... restored, got:\n%s", s)
	}
}
