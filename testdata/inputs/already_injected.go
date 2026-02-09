package sample

import "go.uber.org/zap"

// AlreadyInjected 已注入场景: zap 调用已有正确的注入字段 (需用工具生成正确值后再填)
func AlreadyInjected() {
	zap.L().Info("hello", zap.String("fl", "already_injected.go:8"))
	zap.L().Error("fail", zap.String("fl", "already_injected.go:9"))
}
