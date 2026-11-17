// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/metadata"
)

// NewFactory creates a factory for file_log receiver
func NewFactory() receiver.Factory {
	factory := adapter.NewFactory(ReceiverType{}, metadata.LogsStability,
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
	return xreceiver.NewFactory(metadata.Type, factory.CreateDefaultConfig,
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType),
		xreceiver.WithLogs(func(ctx context.Context, set receiver.Settings, cfg component.Config, next consumer.Logs) (receiver.Logs, error) {
			rcvr, err := factory.CreateLogs(ctx, set, cfg, next)
			if err == nil && set.ID.Type() == metadata.DeprecatedType {
				set.Logger.Warn("The filelog receiver type is deprecated; use file_log and update the pipeline receiver references, preserving the instance name and storage configuration",
					zap.String("migration_guide", "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/filelogreceiver/MIGRATION.md#receiver-type-name"))
			}
			return rcvr, err
		}, metadata.LogsStability),
	)
}

// ReceiverType implements adapter.LogReceiverTypeWithScope
// to create a file tailing receiver
type ReceiverType struct{}

var _ adapter.LogReceiverTypeWithScope = (*ReceiverType)(nil)

// ScopeName sets the scope name that will be used on all logs coming from the receiver.
func (ReceiverType) ScopeName() string { return metadata.ScopeName }

// Type is the receiver type
func (ReceiverType) Type() component.Type {
	return metadata.Type
}

// CreateDefaultConfig creates a config with type and version
func (ReceiverType) CreateDefaultConfig() component.Config {
	return createDefaultConfig()
}

func createDefaultConfig() *FileLogConfig {
	return &FileLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: *file.NewConfig(),
	}
}

// BaseConfig gets the base config from config, for now
func (ReceiverType) BaseConfig(cfg component.Config) adapter.BaseConfig {
	return cfg.(*FileLogConfig).BaseConfig
}

// FileLogConfig defines configuration for the file_log receiver
type FileLogConfig struct {
	InputConfig        file.Config `mapstructure:",squash"`
	adapter.BaseConfig `mapstructure:",squash"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// InputConfig unmarshals the input operator
func (ReceiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(&cfg.(*FileLogConfig).InputConfig)
}
