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

package syslog

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/input/tcp"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/input/udp"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/parser/syslog"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("syslog_input", func() operator.Builder { return NewSyslogInputConfig("") })
}
func NewSyslogInputConfig(operatorID string) *SyslogInputConfig {
	return &SyslogInputConfig{
		InputConfig: helper.NewInputConfig(operatorID, "syslog_input"),
	}
}

type BaseSyslogInputConfig struct {
	helper.InputConfig `yaml:",inline"`
	Tcp                *tcp.TCPInputConfig `json:"tcp" yaml:"tcp"`
	Udp                *udp.UDPInputConfig `json:"udp" yaml:"udp"`
}

type SyslogInputConfig struct {
	syslog.SyslogParserConfig `yaml:"-"`
	helper.InputConfig        `yaml:",inline"`
	Tcp                       *tcp.TCPInputConfig `json:"tcp" yaml:"tcp"`
	Udp                       *udp.UDPInputConfig `json:"udp" yaml:"udp"`
}

func (c SyslogInputConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	if c.Tcp == nil && c.Udp == nil {
		return nil, fmt.Errorf("need tcp config or udp config")
	}

	c.SyslogParserConfig.OutputIDs = c.OutputIDs
	ops, err := c.SyslogParserConfig.Build(context)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve syslog config: %s", err)
	}

	if c.Tcp != nil {
		c.Tcp.OutputIDs = []string{ops[0].ID()}
		inputOps, err := c.Tcp.Build(context)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tcp config: %s", err)
		}
		ops = append(ops, inputOps...)
	}

	if c.Udp != nil {
		c.Udp.OutputIDs = []string{ops[0].ID()}
		inputOps, err := c.Udp.Build(context)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve upd config: %s", err)
		}
		ops = append(ops, inputOps...)
	}

	return ops, nil
}

func (c *SyslogInputConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	parserCfg := syslog.NewSyslogParserConfig("syslog_parser")

	err := unmarshal(parserCfg)
	if err != nil {
		return err
	}
	c.SyslogParserConfig = *parserCfg

	base := &BaseSyslogInputConfig{}
	err = unmarshal(base)
	if err != nil {
		return err
	}

	c.InputConfig = base.InputConfig
	c.Tcp = base.Tcp
	c.Udp = base.Udp
	return nil
}
