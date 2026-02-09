//
// FilePath    : zap-smap\walk.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 目录遍历与文件处理
//

package main

import (
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// runSingleFileMode 处理传入单文件路径的情况
func runSingleFileMode(path string, fSet *token.FileSet, modulePath, baseDir string) error {
	// 跳过不处理的文件类型
	if shouldSkipFile(path) {
		return nil
	}

	// 验证模式
	if *verifyFlg {
		return verifyAndHandleSingleFile(path, fSet, modulePath, baseDir)
	}

	// 处理单个文件的 AST 注入逻辑
	modified, out, modifiedLines, err := processFile(path, fSet, modulePath, baseDir)
	if err != nil {
		return err
	}

	return applyPatchIfModified(path, modified, out, modifiedLines, baseDir)
}

// runDirectoryMode 处理目录遍历模式
func runDirectoryMode(target string, fSet *token.FileSet, modulePath, baseDir string) error {
	if *verifyFlg {
		return runVerifyWalk(target, fSet, modulePath, baseDir)
	}

	return runPatchWalk(target, fSet, modulePath, baseDir)
}

// runPatchWalk 遍历目录并对每个文件执行 AST 注入/写回(非 verify 模式)
func runPatchWalk(target string, fSet *token.FileSet, modulePath, baseDir string) error {
	return filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if shouldSkipDir(path) {
				return filepath.SkipDir
			}

			return nil
		}

		if shouldSkipFile(path) {
			return nil
		}

		modified, out, modifiedLines, err := processFile(path, fSet, modulePath, baseDir)
		if err != nil {
			return err
		}

		return applyPatchIfModified(path, modified, out, modifiedLines, baseDir)
	})
}

// runVerifyWalk 遍历目录并在 verify 模式下收集并打印汇总
func runVerifyWalk(target string, fSet *token.FileSet, modulePath, baseDir string) error {
	var totalAll, missingAll, mismatchAll int

	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if shouldSkipDir(path) {
				return filepath.SkipDir
			}

			return nil
		}

		if shouldSkipFile(path) {
			return nil
		}

		total, missing, mismatch, _, err := reportVerifyForPath(path, fSet, modulePath, baseDir)
		if err != nil {
			return err
		}

		if total > 0 {
			totalAll += total
			missingAll += missing
			mismatchAll += mismatch
		}

		return nil
	})

	if err != nil {
		return err
	}

	printVerifySummary(totalAll, missingAll, mismatchAll)

	return nil
}

// shouldSkipDir 判断目录路径是否应当跳过(例如 vendor/.git 等), 支持 -exclude
func shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	if base == "vendor" || base == ".git" || base == "build" || base == "node_modules" {
		return true
	}

	pathSl := filepath.ToSlash(path)

	for _, ex := range excludeList {
		if strings.Contains(ex, "/") {
			exClean := strings.TrimSuffix(ex, "/")
			if pathSl == exClean || strings.HasPrefix(pathSl+"/", exClean+"/") {
				return true
			}
		} else if base == ex {
			return true
		}
	}

	return false
}

// shouldSkipFile 判断文件路径是否应当跳过(非 go 文件、生成文件或特定 internal 路径), 支持 -exclude
func shouldSkipFile(path string) bool {
	// 非 go 文件
	if !strings.HasSuffix(path, ".go") {
		return true
	}

	// 生成文件或特定 internal 目录, 统一使用 '/' 作为内部判断的分隔符
	pathSl := filepath.ToSlash(path)
	if strings.HasSuffix(pathSl, "_gen.go") || strings.Contains(pathSl, "/internal/") {
		return true
	}

	// 检查用户指定的排除列表

	for _, ex := range excludeList {
		if strings.Contains(ex, "/") {
			exClean := strings.TrimSuffix(ex, "/")
			if pathSl == exClean || strings.HasPrefix(pathSl+"/", exClean+"/") {
				return true
			}
		} else if filepath.Base(path) == ex {
			return true
		}
	}

	return false
}
