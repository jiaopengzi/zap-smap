package sample

import "go.uber.org/zap"

// MultiFunc 多函数场景: 不同函数中有多个 zap 日志调用
func InitApp() {
	zap.L().Info("app started")
	zap.L().Debug("loading config")
}

func HandleRequest() {
	zap.L().Warn("slow response detected")
	zap.L().Error("request failed")
}

func Cleanup() {
	zap.L().Info("shutting down")
}
