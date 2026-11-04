// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor/internal/metadata"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createLogsProcessor(
	ctx context.Context,
	settings processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	processorCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %+v", cfg)
	}

	if err := processorCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	proc, err := newProcessor(ctx, processorCfg, nextConsumer, settings)
	if err != nil {
		return nil, fmt.Errorf("error creating processor: %w", err)
	}

	return proc, nil
}

func newKeyMaterialProvider(_ context.Context, cfg *Config, logger *zap.Logger) (KeyMaterialProvider, error) {
	switch cfg.KeySource.Type {
	case keySourceK8sSecret:
		logger.Info("Kubernetes Secret key material provider is not yet implemented")
		return nil, fmt.Errorf("key_source.type %q is not yet implemented", keySourceK8sSecret)
	case keySourceEnv:
		logger.Info("Initializing key material provider from inline env config (use ${env:VAR} in YAML)")
		return newInlineKeyMaterialProvider(cfg.KeySource.Env)
	case keySourceFile:
		if cfg.KeySource.File.HMACKey != "" {
			logger.Info("Initializing HMAC key material provider from file",
				zap.String("hmac_key", cfg.KeySource.File.HMACKey),
			)
		} else {
			logger.Info("Initializing certificate key material provider from file",
				zap.String("certificate", cfg.KeySource.File.Certificate),
			)
		}
		return newFileKeyMaterialProvider(cfg.KeySource.File)
	case keySourceBao:
		logger.Info("OpenBao key material provider is not yet implemented")
		return nil, fmt.Errorf("key_source.type %q is not yet implemented", keySourceBao)
	default:
		return nil, fmt.Errorf("unknown key_source.type: %q", cfg.KeySource.Type)
	}
}
