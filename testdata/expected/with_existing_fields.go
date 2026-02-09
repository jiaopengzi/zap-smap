package sample

import "go.uber.org/zap"

// WithExistingFields 带现有字段场景: zap 调用已有其他字段但没有注入字段
func WithExistingFields() {
	zap.L().Info("order created", zap.String("fl", "with_existing_fields.go:7"), zap.String("order_id", "12345"), zap.String("user", "alice"))
	zap.L().Error("payment failed", zap.String("fl", "with_existing_fields.go:8"), zap.String("reason", "timeout"), zap.Uint64("amount", 100))
}
