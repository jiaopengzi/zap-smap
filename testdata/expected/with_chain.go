package sample

import "go.uber.org/zap"

// WithChain 链式调用场景: 使用 zap.L().With(...).Info(...)
func WithChain() {
	zap.L().With(zap.String("module", "auth")).Info("user login", zap.String("fl", "with_chain.go:7"))
	zap.L().With(zap.String("module", "db")).Error("query failed", zap.String("fl", "with_chain.go:8"))
}
