// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	operatorType = "udp_input"

	// Maximum UDP packet size
	MaxUDPSize = 64 * 1024

	defaultReaders        = 1
	defaultProcessors     = 1
	defaultMaxQueueLength = 100
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new UDP input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new UDP input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
		BaseConfig: BaseConfig{
			Encoding:        "utf-8",
			OneLogPerPacket: false,
			SplitConfig: split.Config{
				LineEndPattern: ".^", // Use never matching regex to not split data by default
			},
		},
	}
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

// Build will build a udp input operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if c.ListenAddress == "" {
		return nil, errors.New("missing required parameter 'listen_address'")
	}

	address, err := net.ResolveUDPAddr("udp", c.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve listen_address: %w", err)
	}

	enc, err := textutils.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, err
	}

	// Build split func
	splitFunc, err := c.SplitConfig.Func(enc, true, MaxUDPSize)
	if err != nil {
		return nil, err
	}
	splitFunc = trim.WithFunc(splitFunc, c.TrimConfig.Func())

	var resolver *helper.IPResolver
	if c.AddAttributes {
		resolver = helper.NewIPResolver()
	}

	if c.AsyncConfig != nil {
		if c.AsyncConfig.Readers <= 0 {
			c.AsyncConfig.Readers = defaultReaders
		}
		if c.AsyncConfig.Processors <= 0 {
			c.AsyncConfig.Processors = defaultProcessors
		}
		if c.AsyncConfig.MaxQueueLength <= 0 {
			c.AsyncConfig.MaxQueueLength = defaultMaxQueueLength
		}
	}

	udpInput := &Input{
		InputOperator:   inputOperator,
		address:         address,
		buffer:          make([]byte, MaxUDPSize),
		addAttributes:   c.AddAttributes,
		encoding:        enc,
		splitFunc:       splitFunc,
		resolver:        resolver,
		OneLogPerPacket: c.OneLogPerPacket,
		AsyncConfig:     c.AsyncConfig,
	}

	if c.AsyncConfig != nil {
		udpInput.messageQueue = make(chan messageAndAddress, c.AsyncConfig.MaxQueueLength)
		udpInput.readBufferPool = sync.Pool{
			New: func() any {
				buffer := make([]byte, MaxUDPSize)
				return &buffer
			},
		}
	}
	return udpInput, nil
}
