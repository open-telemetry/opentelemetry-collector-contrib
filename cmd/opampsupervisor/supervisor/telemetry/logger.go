package telemetry

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"go.uber.org/zap"
)

func NewLogger(cfg config.Logs) (*zap.Logger, error) {
	zapCfg := zap.NewProductionConfig()

	zapCfg.Level = zap.NewAtomicLevelAt(cfg.Level)
	zapCfg.OutputPaths = cfg.OutputPaths

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	return logger, nil
}
