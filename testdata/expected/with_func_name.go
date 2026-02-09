package sample

import "go.uber.org/zap"

// WithFuncName 函数名注入场景: 用于测试 -with-func 参数
func WithFuncName() {
	zap.L().Info("action performed", zap.String("fl", "with_func_name.go:7"))
	zap.L().Error("something went wrong", zap.String("fl", "with_func_name.go:8"))
}

type Service struct{}

func (s *Service) Start() {
	zap.L().Info("service starting", zap.String("fl", "with_func_name.go:14"))
}

func (s *Service) Stop() {
	zap.L().Warn("service stopping", zap.String("fl", "with_func_name.go:18"))
}
