// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/syslog"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
)

const operatorType = "syslog_input"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
	}
}

type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	syslog.BaseConfig  `mapstructure:",squash"`
	TCP                *tcp.BaseConfig `mapstructure:"tcp"`
	UDP                *udp.BaseConfig `mapstructure:"udp"`
	OnError            string          `mapstructure:"on_error"`
}

func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputBase, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	syslogParserCfg := syslog.NewConfigWithID(inputBase.ID() + "_internal_tcp")
	syslogParserCfg.BaseConfig = c.BaseConfig
	syslogParserCfg.SetID(inputBase.ID() + "_internal_parser")
	syslogParserCfg.OutputIDs = c.OutputIDs
	syslogParserCfg.MaxOctets = c.MaxOctets
	if c.OnError != "" {
		syslogParserCfg.OnError = c.OnError
	}
	syslogParser, err := syslogParserCfg.Build(set)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve syslog config: %w", err)
	}

	if c.TCP != nil {
		tcpInputCfg := tcp.NewConfigWithID(inputBase.ID() + "_internal_tcp")
		tcpInputCfg.AttributerConfig = c.AttributerConfig
		tcpInputCfg.IdentifierConfig = c.IdentifierConfig
		tcpInputCfg.BaseConfig = *c.TCP
		if syslogParserCfg.EnableOctetCounting {
			tcpInputCfg.SplitFuncBuilder = OctetSplitFuncBuilder
		}

		tcpInput, err := tcpInputCfg.Build(set)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tcp config: %w", err)
		}

		tcpInput.SetOutputIDs([]string{syslogParser.ID()})
		if err := tcpInput.SetOutputs([]operator.Operator{syslogParser}); err != nil {
			return nil, fmt.Errorf("failed to set outputs")
		}

		return &Input{
			InputOperator: inputBase,
			tcp:           tcpInput.(*tcp.Input),
			parser:        syslogParser.(*syslog.Parser),
		}, nil
	}

	if c.UDP != nil {
		udpInputCfg := udp.NewConfigWithID(inputBase.ID() + "_internal_udp")
		udpInputCfg.AttributerConfig = c.AttributerConfig
		udpInputCfg.IdentifierConfig = c.IdentifierConfig
		udpInputCfg.BaseConfig = *c.UDP

		// Octet counting and Non-Transparent-Framing are invalid for UDP connections
		if syslogParserCfg.EnableOctetCounting || syslogParserCfg.NonTransparentFramingTrailer != nil {
			return nil, errors.New("octet_counting and non_transparent_framing is not compatible with UDP")
		}

		udpInput, err := udpInputCfg.Build(set)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve udp config: %w", err)
		}

		udpInput.SetOutputIDs([]string{syslogParser.ID()})
		if err := udpInput.SetOutputs([]operator.Operator{syslogParser}); err != nil {
			return nil, fmt.Errorf("failed to set outputs")
		}

		return &Input{
			InputOperator: inputBase,
			udp:           udpInput.(*udp.Input),
			parser:        syslogParser.(*syslog.Parser),
		}, nil
	}

	return nil, fmt.Errorf("need tcp config or udp config")
}
