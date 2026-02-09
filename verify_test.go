//
// FilePath    : zap-smap\verify_test.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 单测
//

package main

import (
	"os"
	"strings"
	"testing"
)

func TestMain_VerifyMode_MissingField(t *testing.T) {
	resetGlobals()
	td := t.TempDir()
	writeFile(t, td, "verify_zap.go", `package sample

import "go.uber.org/zap"

func Foo() {
	zap.L().Info("hello")
}
`)

	*pathFlag = td
	*verifyFlg = true
	os.Args = []string{"cmd"}

	out := captureOutput(func() {
		main()
	})

	if !strings.Contains(out, "missing=") {
		t.Fatalf("expected verify output with missing count, got: %s", out)
	}
	if !strings.Contains(out, "total=") {
		t.Fatalf("expected verify summary, got: %s", out)
	}
}

func TestMain_VerifyMode_PresentFieldCorrect(t *testing.T) {
	resetGlobals()

	td := t.TempDir()
	writeFile(t, td, "verify_present.go", `package sample

import "go.uber.org/zap"

func Foo() { zap.L().Info("hello", zap.String("file:line","verify_present.go:5")) }
`)

	*pathFlag = td
	*verifyFlg = true
	os.Args = []string{"cmd"}

	out := captureOutput(func() {
		main()
	})

	if !strings.Contains(out, "missing: 0") || !strings.Contains(out, "mismatch: 0") {
		t.Fatalf("expected verify summary with zero issues, got: %s", out)
	}
}

func TestMain_VerifyMode_MismatchField(t *testing.T) {
	resetGlobals()

	td := t.TempDir()
	writeFile(t, td, "verify_mismatch.go", `package sample

import "go.uber.org/zap"

func Foo() { zap.L().Info("hello", zap.String("file:line","wrong-value")) }
`)

	*pathFlag = td
	*verifyFlg = true
	os.Args = []string{"cmd"}

	out := captureOutput(func() {
		main()
	})

	if !strings.Contains(out, "mismatch") {
		t.Fatalf("expected mismatch reported, got: %s", out)
	}
}

func TestMain_VerifyMode_WithFuncFlag_Present(t *testing.T) {
	resetGlobals()

	td := t.TempDir()
	writeFile(t, td, "verify_with_func.go", `package sample

import "go.uber.org/zap"

func Foo() { zap.L().Info("hello", zap.String("file:line","verify_with_func.go:5 | sample.Foo")) }
`)

	*pathFlag = td
	*verifyFlg = true
	*funcFlg = true
	os.Args = []string{"cmd"}

	out := captureOutput(func() {
		main()
	})

	if !strings.Contains(out, "missing: 0") || !strings.Contains(out, "mismatch: 0") {
		t.Fatalf("expected verify summary with zero issues for with-func, got: %s", out)
	}
}
