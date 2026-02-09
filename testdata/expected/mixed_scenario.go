package sample

import "go.uber.org/zap"

// MixedScenario 综合场景: 混合多种调用模式
func MixedInit() {
	zap.L().Info("system booting", zap.String("fl", "mixed_scenario.go:7"))
}

func MixedProcess() {
	zap.L().Debug("begin processing", zap.String("fl", "mixed_scenario.go:11"))
	zap.L().Info("processing item", zap.String("fl", "mixed_scenario.go:12"), zap.String("item_id", "X001"))

	handler := func() {
		zap.L().Warn("handler fallback triggered", zap.String("fl", "mixed_scenario.go:15"))
	}
	handler()

	zap.L().Error("processing failed", zap.String("fl", "mixed_scenario.go:19"), zap.String("reason", "disk full"), zap.Uint64("retry", 3))
}

type Worker struct{}

func (w *Worker) Run() {
	zap.L().Info("worker started", zap.String("fl", "mixed_scenario.go:25"))

	go func() {
		zap.L().Error("worker task failed", zap.String("fl", "mixed_scenario.go:28"))
	}()
}
