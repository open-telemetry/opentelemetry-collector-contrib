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

	proc, err := newProcessor(processorCfg, nextConsumer, settings)
	if err != nil {
		return nil, fmt.Errorf("error creating processor: %w", err)
	}

	return proc, nil
}

func newKeyMaterialProvider(ctx context.Context, cfg *Config, logger *zap.Logger) (KeyMaterialProvider, error) {
	switch cfg.KeySource.Type {
	case KeySourceK8sSecret:
		logger.Info("Initializing key material provider from Kubernetes secret",
			zap.String("secret", cfg.KeySource.K8sSecret.Name),
			zap.String("namespace", cfg.KeySource.K8sSecret.Namespace),
		)
		return newK8sKeyMaterialProvider(ctx, cfg.KeySource.K8sSecret, logger)
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
		logger.Info("Initializing key material provider from OpenBao",
			zap.String("secret_path", cfg.KeySource.Bao.SecretPath),
		)
		return newBaoKeyMaterialProvider(ctx, cfg.KeySource.Bao)
	default:
		return nil, fmt.Errorf("unknown key_source.type: %q", cfg.KeySource.Type)
	}
}
