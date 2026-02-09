package sample

import "go.uber.org/zap"

// MultiLevel 多日志等级场景: 覆盖所有 zap 日志方法
func MultiLevel() {
	zap.L().Debug("debug message", zap.String("fl", "multi_level.go:7"))
	zap.L().Info("info message", zap.String("fl", "multi_level.go:8"))
	zap.L().Warn("warn message", zap.String("fl", "multi_level.go:9"))
	zap.L().Error("error message", zap.String("fl", "multi_level.go:10"))
	zap.L().DPanic("dpanic message", zap.String("fl", "multi_level.go:11"))
}
