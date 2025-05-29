// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"go.opentelemetry.io/collector/service/telemetry"
	"go.uber.org/zap"
)

func NewLogger(cfg telemetry.LogsConfig) (*zap.Logger, error) {
	zapCfg := zap.NewProductionConfig()

	zapCfg.Level = zap.NewAtomicLevelAt(cfg.Level)
	zapCfg.OutputPaths = cfg.OutputPaths

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	return logger, nil
}
