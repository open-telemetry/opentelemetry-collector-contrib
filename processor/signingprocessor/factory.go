// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor/internal/metadata"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createLogsProcessor(
	_ context.Context,
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

	proc, err := newProcessor(processorCfg, nextConsumer, settings)
	if err != nil {
		return nil, fmt.Errorf("error creating processor: %w", err)
	}

	return proc, nil
}

func newKeyMaterialProvider(_ context.Context, cfg *Config, logger *zap.Logger) (KeyMaterialProvider, error) {
	switch cfg.KeySource.Type {
	case KeySourceK8sSecret:
		return nil, fmt.Errorf("key_source.type %q not yet implemented", cfg.KeySource.Type)
	case KeySourceEnv:
		if cfg.Algorithm == AlgorithmHMACSHA256 {
			logger.Info("Initializing HMAC key material provider from environment variable",
				zap.String("hmac_key_env_var", cfg.KeySource.Env.HMACKeyEnvVar),
			)
		} else {
			logger.Info("Initializing key material provider from environment variables",
				zap.String("cert_env_var", cfg.KeySource.Env.CertEnvVar),
				zap.String("key_env_var", cfg.KeySource.Env.KeyEnvVar),
			)
		}
		return newEnvKeyMaterialProvider(cfg.KeySource.Env)
	case KeySourceFile:
		if cfg.Algorithm == AlgorithmHMACSHA256 {
			logger.Info("Initializing HMAC key material provider from file",
				zap.String("hmac_key_file", cfg.KeySource.File.HMACKeyFile),
			)
		} else {
			logger.Info("Initializing key material provider from files",
				zap.String("cert_file", cfg.KeySource.File.CertFile),
				zap.String("key_file", cfg.KeySource.File.KeyFile),
			)
		}
		return newFileKeyMaterialProvider(cfg.KeySource.File)
	case KeySourceBao:
		return nil, fmt.Errorf("key_source.type %q not yet implemented", cfg.KeySource.Type)
	default:
		return nil, fmt.Errorf("unknown key_source.type: %q", cfg.KeySource.Type)
	}
}
