//
// FilePath    : zap-smap\types.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 定义工具使用的类型、常量、变量
//

package main

// zapIdent zap 包的标识符
const (
	zapIdent        = "zap"
	zapMethodString = "String"
	zapMethodAny    = "Any"
	zapMethodUint64 = "Uint64"
)

// logMethods 列出要注入的 zap 方法名
var logMethods = map[string]bool{
	"Debug":  true,
	"Info":   true,
	"Warn":   true,
	"Error":  true,
	"DPanic": true,
	"Panic":  true,
	"Fatal":  true,
}

// fnRange 源文件中一个函数的字节范围和名字(用于定位调用所在的函数)
type fnRange struct {
	start int    // 开始字节偏移
	end   int    // 结束字节偏移
	pkg   string // 包名
	name  string // 函数名
}
