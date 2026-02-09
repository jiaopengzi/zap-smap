package sample

import "go.uber.org/zap"

// SortFields 排序字段场景: zap 字段无序, 用于测试 -sort 参数
func SortFields() {
	zap.L().Info("order event", zap.String("a_user", "alice"), zap.String("fl", "sort_fields.go:7"), zap.String("m_action", "buy"), zap.String("z_trace", "abc"))
	zap.L().Error("payment error", zap.String("amount", "100"), zap.String("currency", "USD"), zap.String("fl", "sort_fields.go:8"), zap.String("reason", "timeout"))
}
