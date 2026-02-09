package sample

import (
	"errors"

	"go.uber.org/zap"
)

// FieldsSlice 字段切片场景: 使用 []zap.Field 切片收集字段, 然后通过 fields... 展开传入
func FieldsSlice() {
	fields := []zap.Field{
		zap.String("module", "order"),
	}
	fields = append(fields, zap.Any("request", "data"))
	fields = append(fields, zap.Error(errors.New("something failed")))

	// 打印日志: 使用 fields... 展开
	zap.L().Warn("请求信息", fields...)
}

// FieldsSliceMixed 混合场景: 同一函数中既有 fields... 展开调用, 也有普通的直接调用
func FieldsSliceMixed() {
	// 普通调用 - 应该被正常注入
	zap.L().Info("start processing")

	// fields 切片方式
	fields := []zap.Field{
		zap.String("user_id", "12345"),
		zap.String("action", "purchase"),
	}
	fields = append(fields, zap.Any("detail", "item"))

	// 使用 fields... 展开
	zap.L().Error("处理失败", fields...)

	// 普通调用 - 也应该被正常注入
	zap.L().Info("end processing")
}

// FieldsSliceInline 内联构建切片并展开
func FieldsSliceInline() {
	zap.L().Info("inline fields", []zap.Field{
		zap.String("key1", "val1"),
		zap.String("key2", "val2"),
	}...)
}

// FieldsSliceFromFunc 从函数返回值获取 fields 并展开
func FieldsSliceFromFunc() {
	zap.L().Debug("from func", getFields()...)
}

func getFields() []zap.Field {
	return []zap.Field{
		zap.String("source", "config"),
	}
}

// FieldsSliceConditional 条件构建字段切片
func FieldsSliceConditional() {
	fields := []zap.Field{
		zap.String("step", "validate"),
	}

	ok := true
	if ok {
		fields = append(fields, zap.String("status", "success"))
	} else {
		fields = append(fields, zap.String("status", "failed"))
		fields = append(fields, zap.Error(errors.New("validation error")))
	}

	zap.L().Info("验证结果", fields...)
}

// NormalCallsOnly 纯普通调用(无 fields... ), 作为对照组
func NormalCallsOnly() {
	zap.L().Info("normal call 1")
	zap.L().Error("normal call 2", zap.String("key", "value"))
}
