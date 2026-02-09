package sample

import "go.uber.org/zap"

// MultiFunc 多函数场景: 不同函数中有多个 zap 日志调用
func InitApp() {
	zap.L().Info("app started", zap.String("fl", "multi_func.go:7"))
	zap.L().Debug("loading config", zap.String("fl", "multi_func.go:8"))
}

func HandleRequest() {
	zap.L().Warn("slow response detected", zap.String("fl", "multi_func.go:12"))
	zap.L().Error("request failed", zap.String("fl", "multi_func.go:13"))
}

func Cleanup() {
	zap.L().Info("shutting down", zap.String("fl", "multi_func.go:17"))
}
