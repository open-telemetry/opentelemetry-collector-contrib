// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func NewLogger(cfg config.Logs) (*zap.Logger, error) {
	zapCfg := newZapConfig(cfg)

	zapCfg.Level = zap.NewAtomicLevelAt(cfg.Level)
	zapCfg.OutputPaths = cfg.OutputPaths

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	return logger, nil
}

func newZapConfig(cfg config.Logs) zap.Config {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Encoding = "json"

	if isConsoleLogFormat(cfg.LogFormat) {
		zapCfg.Encoding = "console"
		zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	return zapCfg
}

func isConsoleLogFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "console", "text":
		return true
	default:
		return false
	}
}
