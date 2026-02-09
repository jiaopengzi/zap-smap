package sample

import "go.uber.org/zap"

// DelTarget 删除字段场景: 文件中包含旧字段 "file:line", 用于测试 -del 参数
func DelTarget() {
	zap.L().Info("hello")
	zap.L().Error("fail", zap.String("extra", "data"))
}
