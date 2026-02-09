//
// FilePath    : zap-smap\test_helpers_test.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 单测辅助函数
//

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureOutput 捕获标准输出和标准错误输出的内容, 返回捕获到的字符串
func captureOutput(f func()) string {
	oldOut := os.Stdout
	oldErr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	go func() {
		var buf strings.Builder
		io.Copy(&buf, rOut)
		io.Copy(&buf, rErr)
		outC <- buf.String()
	}()

	f()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr

	// 将 Windows 风格的路径分隔符统一为 '/'，使测试断言与平台无关
	outStr := <-outC
	outStr = strings.ReplaceAll(outStr, "\\", "/")

	return outStr
}

// writeFile 在指定目录 dir 下创建文件 name 并写入内容 content
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

// resetGlobals 重置全局变量, 以避免测试间相互影响
func resetGlobals() {
	*pathFlag = "."
	*fieldFlg = "file:line"
	*writeFlg = false
	*funcFlg = false
	*verifyFlg = false
	*excludeFlag = ""
	*delFlg = ""
	excludeList = nil
}

// reset newly added flags
func resetNewFlags() {
	if positionFlg != nil {
		*positionFlg = -1
	}
	if sortFlg != nil {
		*sortFlg = false
	}
}
