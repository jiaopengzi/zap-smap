package sample

import "go.uber.org/zap"

// MixedScenario 综合场景: 混合多种调用模式
func MixedInit() {
	zap.L().Info("system booting")
}

func MixedProcess() {
	zap.L().Debug("begin processing")
	zap.L().Info("processing item", zap.String("item_id", "X001"))

	handler := func() {
		zap.L().Warn("handler fallback triggered")
	}
	handler()

	zap.L().Error("processing failed", zap.String("reason", "disk full"), zap.Uint64("retry", 3))
}

type Worker struct{}

func (w *Worker) Run() {
	zap.L().Info("worker started")

	go func() {
		zap.L().Error("worker task failed")
	}()
}
