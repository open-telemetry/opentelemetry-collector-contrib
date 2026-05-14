// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namedpipereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/namedpipereceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/namedpipe"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/namedpipereceiver/internal/metadata"
)

// NewFactory creates a factory for the named_pipe receiver.
func NewFactory() receiver.Factory {
	return adapter.NewFactory(&ReceiverType{}, metadata.LogsStability,
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

type ReceiverType struct{}

// Type is the receiver type
func (ReceiverType) Type() component.Type {
	return metadata.Type
}

// CreateDefaultConfig creates a config with type and version
func (ReceiverType) CreateDefaultConfig() component.Config {
	return createDefaultConfig()
}

func createDefaultConfig() *NamedPipeConfig {
	return &NamedPipeConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: func() namedpipe.Config {
			conf := namedpipe.NewConfig()
			conf.Permissions = 0o666

			return *conf
		}(),
	}
}

// BaseConfig gets the base config from config, for now
func (ReceiverType) BaseConfig(cfg component.Config) adapter.BaseConfig {
	return cfg.(*NamedPipeConfig).BaseConfig
}

// NamedPipeConfig defines configuration for the named_pipe receiver
type NamedPipeConfig struct {
	InputConfig        namedpipe.Config `mapstructure:",squash"`
	adapter.BaseConfig `mapstructure:",squash"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// InputConfig unmarshals the input operator
func (ReceiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(&cfg.(*NamedPipeConfig).InputConfig)
}
