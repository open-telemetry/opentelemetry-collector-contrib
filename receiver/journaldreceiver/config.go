// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package journaldreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver/internal/metadata"
)

// createDefaultConfig creates a config with type and version
func createDefaultConfig() component.Config {
	return &JournaldConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: *journald.NewConfig(),
	}
}

// receiverType implements adapter.LogReceiverType
// to create a journald receiver
type receiverType struct{}

// Type is the receiver type
func (receiverType) Type() component.Type {
	return metadata.Type
}

// BaseConfig gets the base config from config, for now
func (receiverType) BaseConfig(cfg component.Config) adapter.BaseConfig {
	return cfg.(*JournaldConfig).BaseConfig
}

// JournaldConfig defines configuration for the journald receiver
type JournaldConfig struct {
	adapter.BaseConfig `mapstructure:",squash"`
	InputConfig        journald.Config `mapstructure:",squash"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// InputConfig unmarshals the input operator
func (receiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(&cfg.(*JournaldConfig).InputConfig)
}

// CreateDefaultConfig creates a config with type and version
func (receiverType) CreateDefaultConfig() component.Config {
	return createDefaultConfig()
}
