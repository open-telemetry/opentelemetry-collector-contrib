// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/syslog"

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

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
}

func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputBase, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	syslogParserCfg := syslog.NewConfigWithID(inputBase.ID() + "_internal_tcp")
	syslogParserCfg.BaseConfig = c.BaseConfig
	syslogParserCfg.SetID(inputBase.ID() + "_internal_parser")
	syslogParserCfg.OutputIDs = c.OutputIDs
	syslogParser, err := syslogParserCfg.Build(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve syslog config: %w", err)
	}

	if c.TCP != nil {
		tcpInputCfg := tcp.NewConfigWithID(inputBase.ID() + "_internal_tcp")
		tcpInputCfg.BaseConfig = *c.TCP
		if syslogParserCfg.EnableOctetCounting {
			tcpInputCfg.SplitFuncBuilder = OctetSplitFuncBuilder
		}

		tcpInput, err := tcpInputCfg.Build(logger)
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
		udpInputCfg.BaseConfig = *c.UDP

		// Octet counting and Non-Transparent-Framing are invalid for UDP connections
		if syslogParserCfg.EnableOctetCounting || syslogParserCfg.NonTransparentFramingTrailer != nil {
			return nil, errors.New("octet_counting and non_transparent_framing is not compatible with UDP")
		}

		udpInput, err := udpInputCfg.Build(logger)
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

// Input is an operator that listens for log entries over tcp.
type Input struct {
	helper.InputOperator
	tcp    *tcp.Input
	udp    *udp.Input
	parser *syslog.Parser
}

// Start will start listening for log entries over tcp or udp.
func (t *Input) Start(p operator.Persister) error {
	if t.tcp != nil {
		return t.tcp.Start(p)
	}
	return t.udp.Start(p)
}

// Stop will stop listening for messages.
func (t *Input) Stop() error {
	if t.tcp != nil {
		return t.tcp.Stop()
	}
	return t.udp.Stop()
}

// SetOutputs will set the outputs of the internal syslog parser.
func (t *Input) SetOutputs(operators []operator.Operator) error {
	t.parser.SetOutputIDs(t.GetOutputIDs())
	return t.parser.SetOutputs(operators)
}

func OctetSplitFuncBuilder(_ encoding.Encoding) (bufio.SplitFunc, error) {
	return newOctetFrameSplitFunc(true), nil
}

func newOctetFrameSplitFunc(flushAtEOF bool) bufio.SplitFunc {
	frameRegex := regexp.MustCompile(`^[1-9]\d*\s`)
	return func(data []byte, atEOF bool) (int, []byte, error) {
		frameLoc := frameRegex.FindIndex(data)
		if frameLoc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil
		}

		frameMaxIndex := frameLoc[1]
		// Remove the delimiter (space) between length and log, and parse the length
		frameLenValue, err := strconv.Atoi(string(data[:frameMaxIndex-1]))
		if err != nil {
			// This should not be possible because the regex matched.
			// However, return an error just in case.
			return 0, nil, err
		}

		advance := frameMaxIndex + frameLenValue
		if advance > len(data) {
			if atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil
		}
		return advance, data[:advance], nil
	}
}
