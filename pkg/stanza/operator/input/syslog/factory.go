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

var operatorType = component.MustNewType("syslog_input")

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
	}
}

func createOperator(set component.TelemetrySettings, cfg component.Config) (operator.Operator, error) {
	c := cfg.(*Config)
	inputBase, err := helper.NewInput(set, c.InputConfig)
	if err != nil {
		return nil, err
	}

	syslogParserFactory := syslog.NewFactory()
	syslogParserCfg := syslogParserFactory.NewDefaultConfig(inputBase.ID() + "_internal_tcp").(*syslog.Config)
	syslogParserCfg.BaseConfig = c.BaseConfig
	syslogParserCfg.SetID(inputBase.ID() + "_internal_parser")
	syslogParserCfg.OutputIDs = c.OutputIDs
	syslogParser, err := syslogParserFactory.CreateOperator(set, syslogParserCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve syslog config: %w", err)
	}

	if c.TCP != nil {
		tcpFactory := tcp.NewFactory()
		tcpInputCfg := tcpFactory.NewDefaultConfig(inputBase.ID() + "_internal_tcp").(*tcp.Config)
		tcpInputCfg.BaseConfig = *c.TCP
		if syslogParserCfg.EnableOctetCounting {
			tcpInputCfg.SplitFuncBuilder = OctetSplitFuncBuilder
		}

		tcpInput, err := tcpFactory.CreateOperator(set, tcpInputCfg)
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
		udpFactory := udp.NewFactory()
		udpInputCfg := udpFactory.NewDefaultConfig(inputBase.ID() + "_internal_udp").(*udp.Config)
		udpInputCfg.BaseConfig = *c.UDP

		// Octet counting and Non-Transparent-Framing are invalid for UDP connections
		if syslogParserCfg.EnableOctetCounting || syslogParserCfg.NonTransparentFramingTrailer != nil {
			return nil, errors.New("octet_counting and non_transparent_framing is not compatible with UDP")
		}

		udpInput, err := udpFactory.CreateOperator(set, udpInputCfg)
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
