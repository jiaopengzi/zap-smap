//
// FilePath    : zap-smap\walk_test.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 单测
//

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_PositionFlag_InsertsAtPosition(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "pos.go", `package sample

import "go.uber.org/zap"

func Foo() { zap.L().Info("hello", zap.String("a","1")) }
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	*positionFlg = 2 // field 索引 2 = msg 之后第 3 个参数(AST 索引 3)
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "pos.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)
	idxA := strings.Index(s, `zap.String("a"`)
	idxFl := strings.Index(s, `zap.String("fl"`)
	if idxA < 0 || idxFl < 0 {
		t.Fatalf("expected both a and fl present, got: %s", s)
	}
	if !(idxA < idxFl) {
		t.Fatalf("expected a before fl when position=2, got order: %s", s)
	}
}

func TestMain_SortFlag_SortsFields(t *testing.T) {
	resetGlobals()
	resetNewFlags()

	td := t.TempDir()
	writeFile(t, td, "sort.go", `package sample

import "go.uber.org/zap"

func Foo() { zap.L().Info("hello", zap.String("z","3"), zap.String("a","1")) }
`)

	*pathFlag = td
	*writeFlg = true
	*fieldFlg = "fl"
	*sortFlg = true
	os.Args = []string{"cmd"}

	_ = captureOutput(func() { main() })

	p := filepath.Join(td, "sort.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	s := string(b)
	idxA := strings.Index(s, `zap.String("a"`)
	idxFl := strings.Index(s, `zap.String("fl"`)
	idxZ := strings.Index(s, `zap.String("z"`)

	if idxA < 0 || idxFl < 0 || idxZ < 0 {
		t.Fatalf("expected a, fl, z present, got: %s", s)
	}
	if !(idxA < idxFl && idxFl < idxZ) {
		t.Fatalf("expected order a < fl < z after sort, got: %s", s)
	}
}
