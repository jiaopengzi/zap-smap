//
// FilePath    : zap-smap\main.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : zap 日志打印注入 file:line 信息的工具
//

// zap 日志打印注入 file:line 信息工具, 在导入了 go.uber.org/zap 的源码文件中.
// 自动在 zap.L().Info/Error/Debug/Warn/... 调用处注入 zap.String("fl","file:line")
// 使用方法(在仓库根目录运行):
//
// 查看帮助:
//
// `go run . -h`
//
//	默认是 dry-run(不修改文件, 展示预览), 加上 -write 才会写回文件
package main

import (
	"flag"
	"fmt"
	"go/token"
	"os"
	"runtime/debug"
)

var (
	// Version 软件版本号, 构建时通过 ldflags 注入
	Version = "dev"

	// Commit 提交 Git Commit Hash, 构建时通过 ldflags 注入
	Commit = "unknown"

	// BuildTime 构建时间, 构建时通过 ldflags 注入
	BuildTime = "unknown"
)

func main() {
	// 尝试使用 build info 填充版本信息(仅当 ldflags 未注入时生效)
	if bi, ok := debug.ReadBuildInfo(); ok {
		applyBuildInfo(bi)
	}

	// 解析命令行参数
	flag.Parse()

	// 检查参数冲突
	if err := checkFlagConflicts(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// 获取目标路径
	target := *pathFlag

	// 创建文件集
	fSet := token.NewFileSet()

	// 规范基准路径(用于计算相对路径)
	baseDir, err := normalizeBaseDir(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to determine base dir: %v\n", err)
		os.Exit(1)
	}

	// 读取 module path(可选), 用于生成完整函数路径
	modulePath := readModulePath(baseDir)

	// 解析 -exclude 参数
	parseExcludeList(baseDir)

	// 支持两种用法, 传入目录(默认)或传入单个文件路径
	if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
		// 单文件模式
		if err := runSingleFileMode(target, fSet, modulePath, baseDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// 目录遍历模式
		if err := runDirectoryMode(target, fSet, modulePath, baseDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

// applyBuildInfo 根据 debug.BuildInfo 填充版本信息。
// 优先保留通过 ldflags 已设置的值(非默认值), 只有在默认值时才使用 build info。
func applyBuildInfo(bi *debug.BuildInfo) {
	if bi == nil {
		return
	}

	applyVersion(bi.Main.Version)
	applyVCSSettings(bi.Settings)
}

// applyVersion 如果 Version 仍为默认值, 则使用 build info 中的版本
func applyVersion(v string) {
	if !isDefaultVersion(Version) {
		return
	}

	if v != "" && v != "(devel)" {
		Version = v
	}
}

// isDefaultVersion 判断版本是否为默认值
func isDefaultVersion(v string) bool {
	return v == "dev" || v == ""
}

// isDefaultValue 判断值是否为默认值
func isDefaultValue(v string) bool {
	return v == "unknown" || v == ""
}

// applyVCSSettings 从 build settings 中提取 vcs.revision 和 vcs.time
func applyVCSSettings(settings []debug.BuildSetting) {
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			applyCommit(s.Value)
		case "vcs.time":
			applyBuildTime(s.Value)
		}
	}
}

// applyCommit 如果 Commit 为默认值, 则使用给定值
func applyCommit(v string) {
	if isDefaultValue(Commit) && v != "" {
		Commit = v
	}
}

// applyBuildTime 如果 BuildTime 为默认值, 则使用给定值
func applyBuildTime(v string) {
	if isDefaultValue(BuildTime) && v != "" {
		BuildTime = v
	}
}
