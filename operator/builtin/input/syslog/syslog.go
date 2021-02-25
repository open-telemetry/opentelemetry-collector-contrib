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

type SyslogInputConfig struct {
	helper.InputConfig `yaml:",inline"`
	Tcp                *tcp.TCPInputConfig        `json:"tcp" yaml:"tcp"`
	Udp                *udp.UDPInputConfig        `json:"udp" yaml:"udp"`
	Syslog             *syslog.SyslogParserConfig `json:"syslog" yaml:"syslog"`
}

func (c SyslogInputConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	if c.Syslog == nil {
		return nil, fmt.Errorf("need syslog config")
	}
	if c.Tcp == nil && c.Udp == nil {
		return nil, fmt.Errorf("need tcp config or udp config")
	}

	c.Syslog.OutputIDs = c.OutputIDs
	ops, err := c.Syslog.Build(context)
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
