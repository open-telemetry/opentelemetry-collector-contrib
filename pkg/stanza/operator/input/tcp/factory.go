// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"

import (
	"fmt"
	"net"
	"time"

	"github.com/jpillora/backoff"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

var operatorType = component.MustNewType("tcp_input")

func init() {
	operator.RegisterFactory(NewFactory())
}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType.String()),
		BaseConfig: BaseConfig{
			OneLogPerPacket: false,
			Encoding:        "utf-8",
		},
	}
}

func createOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	inputOperator, err := helper.NewInput(c.InputConfig, set)
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
		return nil, fmt.Errorf("missing required parameter 'listen_address'")
	}

	// validate the input address
	if _, err = net.ResolveTCPAddr("tcp", c.ListenAddress); err != nil {
		return nil, fmt.Errorf("failed to resolve listen_address: %w", err)
	}

	enc, err := decode.LookupEncoding(c.Encoding)
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
		tcpInput.tls, err = c.TLS.LoadTLSConfig()
		if err != nil {
			return nil, err
		}
	}

	return tcpInput, nil
}
