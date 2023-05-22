// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package journaldreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver/internal/metadata"
)

// NewFactory creates a factory for journald receiver
func NewFactory() receiver.Factory {
	return adapter.NewFactory(ReceiverType{}, metadata.LogsStability)
}

// ReceiverType implements adapter.LogReceiverType
// to create a journald receiver
type ReceiverType struct{}

// Type is the receiver type
func (f ReceiverType) Type() component.Type {
	return metadata.Type
}

// CreateDefaultConfig creates a config with type and version
func (f ReceiverType) CreateDefaultConfig() component.Config {
	return &JournaldConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: *journald.NewConfig(),
	}
}

// BaseConfig gets the base config from config, for now
func (f ReceiverType) BaseConfig(cfg component.Config) adapter.BaseConfig {
	return cfg.(*JournaldConfig).BaseConfig
}

// JournaldConfig defines configuration for the journald receiver
type JournaldConfig struct {
	adapter.BaseConfig `mapstructure:",squash"`
	InputConfig        journald.Config `mapstructure:",squash"`
}

// InputConfig unmarshals the input operator
func (f ReceiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(&cfg.(*JournaldConfig).InputConfig)
}
