package sample

import "go.uber.org/zap"

// AnonymousFunc 匿名函数场景: zap 调用在匿名函数内部
func OuterFunc() {
	zap.L().Info("outer call")

	fn := func() {
		zap.L().Info("inside anonymous func")
	}
	fn()

	go func() {
		zap.L().Error("inside goroutine anonymous func")
	}()
}
