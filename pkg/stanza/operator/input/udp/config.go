// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"

import (
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	// Maximum UDP packet size
	MaxUDPSize = 64 * 1024

	defaultReaders        = 1
	defaultProcessors     = 1
	defaultMaxQueueLength = 100
)

// NewConfig creates a new UDP input config with default values
func NewConfig() *Config {
	return NewFactory().NewDefaultConfig(operatorType.String()).(*Config)
}

// NewConfigWithID creates a new UDP input config with default values
func NewConfigWithID(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
}

// Config is the configuration of a udp input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

type AsyncConfig struct {
	Readers        int `mapstructure:"readers,omitempty"`
	Processors     int `mapstructure:"processors,omitempty"`
	MaxQueueLength int `mapstructure:"max_queue_length,omitempty"`
}

// BaseConfig is the details configuration of a udp input operator.
type BaseConfig struct {
	ListenAddress   string       `mapstructure:"listen_address,omitempty"`
	OneLogPerPacket bool         `mapstructure:"one_log_per_packet,omitempty"`
	AddAttributes   bool         `mapstructure:"add_attributes,omitempty"`
	Encoding        string       `mapstructure:"encoding,omitempty"`
	SplitConfig     split.Config `mapstructure:"multiline,omitempty"`
	TrimConfig      trim.Config  `mapstructure:",squash"`
	AsyncConfig     *AsyncConfig `mapstructure:"async,omitempty"`
}

// Deprecated [v0.97.0] Use Factory.CreateOperator instead.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	set := component.TelemetrySettings{}
	if logger != nil {
		set.Logger = logger.Desugar()
	}
	return NewFactory().CreateOperator(&c, set)
}
