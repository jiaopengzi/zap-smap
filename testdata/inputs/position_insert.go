package sample

import "go.uber.org/zap"

// PositionInsert 位置插入场景: 用于测试 -position 参数在不同位置插入字段
func PositionInsert() {
	zap.L().Info("msg", zap.String("a", "1"), zap.String("b", "2"), zap.String("c", "3"))
}
