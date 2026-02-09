//
// FilePath    : zap-smap\main_test.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 单元测试
//

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_DelFlag_RemovesOldField(t *testing.T) {
	resetGlobals()

	td := t.TempDir()
	// file contains old key "file:line" to be deleted
	writeFile(t, td, "with_both.go", `package sample

import "go.uber.org/zap"

func Foo() { zap.L().Info("hello", zap.String("file:line","with_both.go:5")) }
`)

	*pathFlag = td
	*writeFlg = true
	*delFlg = "file:line"
	os.Args = []string{"cmd"}

	out := captureOutput(func() {
		main()
	})

	// Ensure patch was applied
	if !strings.Contains(out, "[PATCH]") {
		t.Fatalf("expected patch output but not found: %s", out)
	}

	// Read file content and ensure old key removed
	p := filepath.Join(td, "with_both.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)
	if strings.Contains(s, "file:line") {
		t.Fatalf("expected file:line removed but still present: %s", s)
	}
}

func TestMain_NoZap_NoPatch(t *testing.T) {
	resetGlobals()

	td := t.TempDir()
	writeFile(t, td, "no_zap.go", `package sample

import "fmt"

func Foo() {
	fmt.Println("hello")
}
`)

	*pathFlag = td
	// ensure test flags don't interfere
	os.Args = []string{"cmd"}

	out := captureOutput(func() {
		main()
	})

	if strings.Contains(out, "[PATCH]") {
		t.Fatalf("unexpected patch output for file without zap import: %s", out)
	}
}

func TestMain_WithZap_DryRunProducesPatch(t *testing.T) {
	resetGlobals()

	td := t.TempDir()
	writeFile(t, td, "with_zap.go", `package sample

import "go.uber.org/zap"

func Foo() {
	zap.L().Info("hello")
}
`)

	*pathFlag = td
	*writeFlg = false
	os.Args = []string{"cmd"}

	out := captureOutput(func() {
		main()
	})

	if !strings.Contains(out, "[PATCH]") {
		t.Fatalf("expected patch output but not found: %s", out)
	}
	if !strings.Contains(out, "preview") {
		t.Fatalf("expected preview in output but not found: %s", out)
	}
}
