package sample

import "go.uber.org/zap"

// Mismatch 值不匹配场景: 注入字段存在但值不正确 (用于 -verify 测试)
func Mismatch() {
	zap.L().Info("hello", zap.String("fl", "wrong_file.go:999"))
	zap.L().Error("fail", zap.String("fl", "incorrect_value"))
}
