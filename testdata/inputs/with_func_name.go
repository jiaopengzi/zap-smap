package sample

import "go.uber.org/zap"

// WithFuncName 函数名注入场景: 用于测试 -with-func 参数
func WithFuncName() {
	zap.L().Info("action performed")
	zap.L().Error("something went wrong")
}

type Service struct{}

func (s *Service) Start() {
	zap.L().Info("service starting")
}

func (s *Service) Stop() {
	zap.L().Warn("service stopping")
}
