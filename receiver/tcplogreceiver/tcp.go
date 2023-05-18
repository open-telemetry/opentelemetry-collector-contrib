// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcplogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver/internal/metadata"
)

// NewFactory creates a factory for tcp receiver
func NewFactory() receiver.Factory {
	return adapter.NewFactory(ReceiverType{}, metadata.LogsStability)
}

// ReceiverType implements adapter.LogReceiverType
// to create a tcp receiver
type ReceiverType struct{}

// Type is the receiver type
func (f ReceiverType) Type() component.Type {
	return metadata.Type
}

// CreateDefaultConfig creates a config with type and version
func (f ReceiverType) CreateDefaultConfig() component.Config {
	return &TCPLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
		InputConfig: *tcp.NewConfig(),
	}
}

// BaseConfig gets the base config from config, for now
func (f ReceiverType) BaseConfig(cfg component.Config) adapter.BaseConfig {
	return cfg.(*TCPLogConfig).BaseConfig
}

// TCPLogConfig defines configuration for the tcp receiver
type TCPLogConfig struct {
	InputConfig        tcp.Config `mapstructure:",squash"`
	adapter.BaseConfig `mapstructure:",squash"`
}

// InputConfig unmarshals the input operator
func (f ReceiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(&cfg.(*TCPLogConfig).InputConfig)
}
