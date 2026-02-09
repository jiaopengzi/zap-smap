package sample

import "go.uber.org/zap"

// Foo 基本场景: 单个函数中有一个 zap 日志调用, 没有注入字段
func Foo() {
	zap.L().Info("hello world", zap.String("fl", "basic.go:7"))
}
