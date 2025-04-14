// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jpillora/backoff"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	operatorType = "tcp_input"

	// minMaxLogSize is the minimal size which can be used for buffering
	// TCP input
	minMaxLogSize = 64 * 1024

	// DefaultMaxLogSize is the max buffer sized used
	// if MaxLogSize is not set
	DefaultMaxLogSize = 1024 * 1024
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new TCP input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new TCP input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
		BaseConfig: BaseConfig{
			OneLogPerPacket: false,
			Encoding:        "utf-8",
		},
	}
}

// Config is the configuration of a tcp input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

// BaseConfig is the detailed configuration of a tcp input operator.
type BaseConfig struct {
	MaxLogSize       helper.ByteSize         `mapstructure:"max_log_size,omitempty"`
	ListenAddress    string                  `mapstructure:"listen_address,omitempty"`
	TLS              *configtls.ServerConfig `mapstructure:"tls,omitempty"`
	AddAttributes    bool                    `mapstructure:"add_attributes,omitempty"`
	OneLogPerPacket  bool                    `mapstructure:"one_log_per_packet,omitempty"`
	Encoding         string                  `mapstructure:"encoding,omitempty"`
	SplitConfig      split.Config            `mapstructure:"multiline,omitempty"`
	TrimConfig       trim.Config             `mapstructure:",squash"`
	SplitFuncBuilder SplitFuncBuilder
}

type SplitFuncBuilder func(enc encoding.Encoding) (bufio.SplitFunc, error)

func (c Config) defaultSplitFuncBuilder(enc encoding.Encoding) (bufio.SplitFunc, error) {
	return c.SplitConfig.Func(enc, true, int(c.MaxLogSize))
}

// Build will build a tcp input operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	// If MaxLogSize not set, set sane default
	if c.MaxLogSize == 0 {
		c.MaxLogSize = DefaultMaxLogSize
	}

	if c.MaxLogSize < minMaxLogSize {
		return nil, fmt.Errorf("invalid value for parameter 'max_log_size', must be equal to or greater than %d bytes", minMaxLogSize)
	}

	if c.ListenAddress == "" {
		return nil, errors.New("missing required parameter 'listen_address'")
	}

	// validate the input address
	if _, err = net.ResolveTCPAddr("tcp", c.ListenAddress); err != nil {
		return nil, fmt.Errorf("failed to resolve listen_address: %w", err)
	}

	enc, err := textutils.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, err
	}

	if c.SplitFuncBuilder == nil {
		c.SplitFuncBuilder = c.defaultSplitFuncBuilder
	}

	// Build split func
	splitFunc, err := c.SplitFuncBuilder(enc)
	if err != nil {
		return nil, err
	}
	splitFunc = trim.WithFunc(splitFunc, c.TrimConfig.Func())

	var resolver *helper.IPResolver
	if c.AddAttributes {
		resolver = helper.NewIPResolver()
	}

	tcpInput := &Input{
		InputOperator:   inputOperator,
		address:         c.ListenAddress,
		MaxLogSize:      int(c.MaxLogSize),
		addAttributes:   c.AddAttributes,
		OneLogPerPacket: c.OneLogPerPacket,
		encoding:        enc,
		splitFunc:       splitFunc,
		backoff: backoff.Backoff{
			Max: 3 * time.Second,
		},
		resolver: resolver,
	}

	if c.TLS != nil {
		tcpInput.tls, err = c.TLS.LoadTLSConfig(context.Background())
		if err != nil {
			return nil, err
		}
	}

	return tcpInput, nil
}
