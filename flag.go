//
// FilePath    : zap-smap\flag.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 命令行参数定义与处理
//

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 命令行参数定义
var (
	pathFlag    = flag.String("path", ".", "要扫描的文件或目录, 默认为项目根目录")
	fieldFlg    = flag.String("field", "fl", "要注入的字段名, 例如 file:line 或 log_site") // fl 表示 file:line 的简写
	delFlg      = flag.String("del", "", "要删除的字段名, 例如 file:line")
	writeFlg    = flag.Bool("write", false, "将修改写回文件")
	funcFlg     = flag.Bool("with-func", false, "在注入内容中包含函数名")
	verifyFlg   = flag.Bool("verify", false, "仅校验注入是否正确(不写回文件), 返回汇总报告")
	excludeFlag = flag.String("exclude", "", "以逗号分隔的要排除的目录或文件路径")
	positionFlg = flag.Int("position", -1, "插入字段的位置索引(0-based)相对于 field 参数列表(跳过 msg); 0=第一个 field 之前, 默认-1等同于0")
	sortFlg     = flag.Bool("sort", false, "按字段键的字母顺序排列 zap 字段")
)

// excludeList 用户指定的排除路径列表
var excludeList []string

// checkFlagConflicts 检查命令行 flag 冲突
func checkFlagConflicts() error {
	var delSet, fieldSet bool

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "del" {
			delSet = true
		}

		if f.Name == "field" {
			fieldSet = true
		}
	})

	// 如果用户显式同时设置了 -del 与 -field, 返回错误
	if delSet && fieldSet {
		return fmt.Errorf("cannot use -del and -field at the same time; remove one flag or let -field use its default")
	}

	return nil
}

// parseExcludeList 将 -exclude 参数解析为可匹配的条目
func parseExcludeList(baseDir string) {
	if *excludeFlag == "" {
		return
	}

	parts := strings.SplitSeq(*excludeFlag, ",")
	for p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// 统一清理路径并使用 '/' 作为分隔符以便匹配
		if strings.Contains(p, string(os.PathSeparator)) || strings.Contains(p, "/") {
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}

			p = filepath.Clean(p)
			excludeList = append(excludeList, filepath.ToSlash(p))
		} else {
			// 仅为简单名称(例如 node_modules)
			excludeList = append(excludeList, p)
		}
	}
}
