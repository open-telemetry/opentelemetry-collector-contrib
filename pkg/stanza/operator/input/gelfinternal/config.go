// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gelfinternal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"

import (
	"fmt"
	"net"
	"sync"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	operatorType = "gelf_input"
	defaultReaders           = 1
	defaultProcessors        = 1
	defaultUDPMaxQueueLength = 100
	defaultListenAddress     = "127.0.0.1:31250"
	defaultProtocol          = "udp"
	MaxUDPSize               = 64 * 1024
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
			ListenAddress:     string(defaultListenAddress),
			Protocol:          string(defaultProtocol),
			AsyncReaders:      defaultReaders,
			AsyncProcessors:   defaultProcessors,
			UDPMaxQueueLength: defaultUDPMaxQueueLength,
		},
	}
}

// Config is the configuration of a udp input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

// BaseConfig is the details configuration of a udp input operator.
type BaseConfig struct {
	ListenAddress     string `mapstructure:"listen_address,omitempty"`
	Protocol          string `mapstructure:"protocol,omitempty"`
	AsyncReaders      int    `mapstructure:"async_readers,omitempty"`
	AsyncProcessors   int    `mapstructure:"async_processors,omitempty"`
	UDPMaxQueueLength int    `mapstructure:"udp_max_queue_length,omitempty"`
}

// Build will build a udp input operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if c.ListenAddress == "" {
		return nil, fmt.Errorf("missing required parameter 'listen_address'")
	}

	if _, _, err := net.SplitHostPort(c.ListenAddress); err != nil {
		return nil, fmt.Errorf("invalid listen_address: %w", err)
	}

	if c.Protocol != "udp" {
		return nil, fmt.Errorf("supported protocols - udp, invalid protocol: %s", c.Protocol)
	}
	if c.AsyncReaders < 1 {
		return nil, fmt.Errorf("invalid async_reader: %d", c.AsyncReaders)
	}
	if c.AsyncProcessors < 1 {
		return nil, fmt.Errorf("invalid async_processors: %d", c.AsyncProcessors)
	}
	if c.UDPMaxQueueLength <= 0 || c.UDPMaxQueueLength > 65535 {
		return nil, fmt.Errorf("expecting queue length greater than 0 and less than 65535, invalid udp_max_queue_length: %d", c.UDPMaxQueueLength)
	}

	udpInput := &Input{
		InputOperator:   inputOperator,
		address:         c.ListenAddress,
		protocol:        c.Protocol,
		udpMessageQueue: make(chan UDPMessage, c.UDPMaxQueueLength),
		readBufferPool: sync.Pool{
			New: func() any {
				buffer := make([]byte, MaxUDPSize)
				return &buffer
			},
		},
		buffer:          make(map[string]*MapGelfMessage),
		lastBuffer:      make(map[string]*MapGelfMessage),
		asyncReaders:    c.AsyncReaders,
		asyncProcessors: c.AsyncProcessors,
	}

	return udpInput, nil
}
