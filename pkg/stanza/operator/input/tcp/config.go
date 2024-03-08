// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"

import (
	"bufio"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	// minMaxLogSize is the minimal size which can be used for buffering
	// TCP input
	minMaxLogSize = 64 * 1024

	// DefaultMaxLogSize is the max buffer sized used
	// if MaxLogSize is not set
	DefaultMaxLogSize = 1024 * 1024
)

// Deprecated [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfig() *Config {
	return NewFactory().NewDefaultConfig(operatorType.String()).(*Config)
}

// Deprecated [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfigWithID(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
}

// Config is the configuration of a tcp input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

// BaseConfig is the detailed configuration of a tcp input operator.
type BaseConfig struct {
	MaxLogSize       helper.ByteSize             `mapstructure:"max_log_size,omitempty"`
	ListenAddress    string                      `mapstructure:"listen_address,omitempty"`
	TLS              *configtls.TLSServerSetting `mapstructure:"tls,omitempty"`
	AddAttributes    bool                        `mapstructure:"add_attributes,omitempty"`
	OneLogPerPacket  bool                        `mapstructure:"one_log_per_packet,omitempty"`
	Encoding         string                      `mapstructure:"encoding,omitempty"`
	SplitConfig      split.Config                `mapstructure:"multiline,omitempty"`
	TrimConfig       trim.Config                 `mapstructure:",squash"`
	SplitFuncBuilder SplitFuncBuilder
}

type SplitFuncBuilder func(enc encoding.Encoding) (bufio.SplitFunc, error)

func (c Config) defaultSplitFuncBuilder(enc encoding.Encoding) (bufio.SplitFunc, error) {
	return c.SplitConfig.Func(enc, true, int(c.MaxLogSize))
}

// Deprecated [v0.97.0] Use Factory.CreateOperator instead.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	set := component.TelemetrySettings{}
	if logger != nil {
		set.Logger = logger.Desugar()
	}
	return NewFactory().CreateOperator(&c, set)
}
