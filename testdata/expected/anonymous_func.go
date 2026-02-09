package sample

import "go.uber.org/zap"

// AnonymousFunc 匿名函数场景: zap 调用在匿名函数内部
func OuterFunc() {
	zap.L().Info("outer call", zap.String("fl", "anonymous_func.go:7"))

	fn := func() {
		zap.L().Info("inside anonymous func", zap.String("fl", "anonymous_func.go:10"))
	}
	fn()

	go func() {
		zap.L().Error("inside goroutine anonymous func", zap.String("fl", "anonymous_func.go:15"))
	}()
}
