// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/syslog"

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
)

func init() {
	operator.Register("syslog_input", func() operator.Builder { return NewInputConfig("") })
}
func NewInputConfig(operatorID string) *InputConfig {
	return &InputConfig{
		InputConfig: helper.NewInputConfig(operatorID, "syslog_input"),
	}
}

type InputConfig struct {
	helper.InputConfig `yaml:",inline"`
	syslog.BaseConfig  `yaml:",inline"`
	TCPConfig          *tcp.BaseConfig `json:"tcp" yaml:"tcp"`
	UDPConfig          *udp.BaseConfig `json:"udp" yaml:"udp"`
}

func (c InputConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputBase, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	syslogParserCfg := syslog.NewParserConfig(inputBase.ID() + "_internal_tcp")
	syslogParserCfg.BaseConfig = c.BaseConfig
	syslogParserCfg.SetID(inputBase.ID() + "_internal_parser")
	syslogParserCfg.OutputIDs = c.OutputIDs
	syslogParser, err := syslogParserCfg.Build(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve syslog config: %s", err)
	}

	if c.TCPConfig != nil {
		tcpInputCfg := tcp.NewInputConfig(inputBase.ID() + "_internal_tcp")
		tcpInputCfg.BaseConfig = *c.TCPConfig

		tcpInput, err := tcpInputCfg.Build(logger)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tcp config: %s", err)
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

	if c.UDPConfig != nil {
		udpInputCfg := udp.NewInputConfig(inputBase.ID() + "_internal_udp")
		udpInputCfg.BaseConfig = *c.UDPConfig

		udpInput, err := udpInputCfg.Build(logger)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve upd config: %s", err)
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
