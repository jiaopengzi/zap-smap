//
// FilePath    : zap-smap\preview.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 处理文件修改预览与写回
//

package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// applyPatchIfModified 将修改写回文件或打印预览, 基于 -write 标志。
func applyPatchIfModified(path string, modified bool, out string, modifiedLines []int, baseDir string) error {
	if !modified {
		return nil
	}

	rel := relPath(path, baseDir)

	fmt.Printf("[PATCH] %s\n", rel)

	if *writeFlg {
		// 写回文件
		if err := os.WriteFile(path, []byte(out), 0600); err != nil {
			return err
		}

		// 使用 gofmt 格式化写回的文件
		if err := runGoFmt(path); err != nil {
			fmt.Fprintf(os.Stderr, "warn: gofmt %s failed: %v\n", rel, err)
		}
	} else {
		// dry-run: 打印每个注入点附近的片段，使用相对于 baseDir 的路径
		printPreview(out, rel, modifiedLines)
	}

	return nil
}

// printPreview 打印 dry-run 模式下的文件修改预览片段
func printPreview(out, path string, modifiedLines []int) {
	fmt.Printf("--- preview (%s) ---\n", path)

	lines := strings.Split(out, "\n")

	if len(modifiedLines) == 0 {
		// 兜底:如果没有记录到行号, 则打印前 40 行
		for i := 0; i < len(lines) && i < 40; i++ {
			fmt.Println(lines[i])
		}

		if len(lines) > 40 {
			fmt.Println("... (truncated) ...")
		}
	} else {
		// 去重并排序行号
		unique := make(map[int]bool)

		var numList []int

		for _, l := range modifiedLines {
			if !unique[l] {
				unique[l] = true

				numList = append(numList, l)
			}
		}

		sort.Ints(numList)

		for _, l := range numList {
			start := max(l-3, 1)

			end := min(l+3, len(lines))

			fmt.Printf("--- snippet around L%d (%s) ---\n", l, path)

			for i := start; i <= end; i++ {
				fmt.Printf("%5d: %s\n", i, lines[i-1])
			}
		}
	}

	fmt.Println("--- end preview ---")
}

// runGoFmt 对指定文件运行 gofmt 格式化。
// 如果 gofmt 不在 PATH 中, 输出警告并返回 nil。
func runGoFmt(path string) error {
	gofmt, err := exec.LookPath("gofmt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "warn: gofmt not found in PATH, skipping format")
		return nil
	}

	cmd := exec.Command(gofmt, "-w", path)
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
