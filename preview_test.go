//
// FilePath    : zap-smap\preview_test.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 单测
//

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestApplyPatchIfModified_NotModified_NoOutput(t *testing.T) {
	*writeFlg = false
	outStr := "line1\nline2\n"
	got := captureOutput(func() {
		if err := applyPatchIfModified("some/path", false, outStr, nil, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if got != "" {
		t.Fatalf("expected no output, got: %q", got)
	}
}

func TestApplyPatchIfModified_DryRun_PrintsPreviewFirstLines(t *testing.T) {
	*writeFlg = false

	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "L" + strconv.Itoa(i+1)
	}
	outStr := strings.Join(lines, "\n")

	out := captureOutput(func() {
		if err := applyPatchIfModified("/tmp/example.txt", true, outStr, nil, "/tmp"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "[PATCH] example.txt") {
		t.Fatalf("missing patch header, got: %q", out)
	}
	if !strings.Contains(out, "--- preview (example.txt) ---") {
		t.Fatalf("missing preview header, got: %q", out)
	}
	// ensure some lines printed
	for i := 1; i <= 3; i++ {
		if !strings.Contains(out, "L"+strconv.Itoa(i)) {
			t.Fatalf("missing line L%d in preview", i)
		}
	}
}

func TestPrintPreview_WithModifiedLines_SnippetIncludesContext(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line " + strconv.Itoa(i+1)
	}
	outStr := strings.Join(lines, "\n")

	out := captureOutput(func() {
		printPreview(outStr, "p.go", []int{5, 15, 5})
	})

	// should contain snippet headers for lines 5 and 15 only once each and numbered lines
	if strings.Count(out, "snippet around L5") != 1 {
		t.Fatalf("expected one snippet around L5, got:\n%s", out)
	}
	if strings.Count(out, "snippet around L15") != 1 {
		t.Fatalf("expected one snippet around L15, got:\n%s", out)
	}
	if !strings.Contains(out, "    5: line 5") {
		t.Fatalf("expected numbered line for 5")
	}
	if !strings.Contains(out, "   15: line 15") {
		t.Fatalf("expected numbered line for 15")
	}
}

func TestApplyPatchIfModified_WriteFlag_WritesFile(t *testing.T) {
	*writeFlg = true

	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	content := "hello\nworld\n"

	// Ensure file does not exist.
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("temp file unexpectedly exists")
	}

	if err := applyPatchIfModified(path, true, content, nil, dir); err != nil {
		t.Fatalf("applyPatchIfModified returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(got) != content {
		t.Fatalf("file content mismatch: got %q want %q", string(got), content)
	}
}

// strconv import used in tests
func init() {
	// ensure imports used
	_ = sort.Ints
	_ = strconv.Itoa
}
