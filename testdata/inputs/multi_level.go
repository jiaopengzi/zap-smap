package sample

import "go.uber.org/zap"

// MultiLevel 多日志等级场景: 覆盖所有 zap 日志方法
func MultiLevel() {
	zap.L().Debug("debug message")
	zap.L().Info("info message")
	zap.L().Warn("warn message")
	zap.L().Error("error message")
	zap.L().DPanic("dpanic message")
}
