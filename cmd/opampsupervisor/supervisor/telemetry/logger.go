// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func NewLogger(cfg config.Logs) (*zap.Logger, error) {
	zapCfg, err := newZapConfig(cfg)
	if err != nil {
		return nil, err
	}

	zapCfg.Level = zap.NewAtomicLevelAt(cfg.Level)
	zapCfg.OutputPaths = cfg.OutputPaths

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	return logger, nil
}

func newZapConfig(cfg config.Logs) (zap.Config, error) {
	zapCfg := zap.NewProductionConfig()

	enc := strings.ToLower(strings.TrimSpace(cfg.Encoding))
	if enc == "" {
		enc = "json"
	}

	switch enc {
	case "console":
		zapCfg.Encoding = "console"
		zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	case "json":
		zapCfg.Encoding = "json"
	default:
		return zap.Config{}, fmt.Errorf("unsupported log encoding %q, supported values are 'json' and 'console'", cfg.Encoding)
	}

	return zapCfg, nil
}
