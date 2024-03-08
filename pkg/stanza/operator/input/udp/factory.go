// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"

import (
	"fmt"
	"net"
	"sync"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

var operatorType = component.MustNewType("udp_input")

func init() {
	operator.RegisterFactory(NewFactory())
}

type factory struct{}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return &factory{}
}

// Type gets the type of the operator.
func (f *factory) Type() component.Type {
	return operatorType
}

// NewDefaultConfig creates a new default configuration.
func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType.String()),
		BaseConfig: BaseConfig{
			Encoding:        "utf-8",
			OneLogPerPacket: false,
			SplitConfig: split.Config{
				LineEndPattern: ".^", // Use never matching regex to not split data by default
			},
		},
	}
}

// CreateOperator creates a UDP input operator.
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	inputOperator, err := helper.NewInput(c.InputConfig, set)
	if err != nil {
		return nil, err
	}

	if c.ListenAddress == "" {
		return nil, fmt.Errorf("missing required parameter 'listen_address'")
	}

	address, err := net.ResolveUDPAddr("udp", c.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve listen_address: %w", err)
	}

	enc, err := decode.LookupEncoding(c.Encoding)
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
