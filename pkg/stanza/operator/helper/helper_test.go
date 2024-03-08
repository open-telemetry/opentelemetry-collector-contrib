// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package helper

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

var helpersTestType = component.MustNewType("helpers_test")

func init() {
	operator.RegisterFactory(newHelpersFactory())
}

type helpersConfig struct {
	WriterConfig `mapstructure:",squash"`
	Time         TimeParser      `mapstructure:"time"`
	Severity     SeverityConfig  `mapstructure:"severity"`
	Scope        ScopeNameParser `mapstructure:"scope"`
	Size         ByteSize        `mapstructure:"size"`
}

type helpersFactory struct{}

func newHelpersFactory() helpersFactory {
	return helpersFactory{}
}

func (f helpersFactory) Type() component.Type {
	return helpersTestType
}

func (f helpersFactory) NewDefaultConfig(operatorID string) component.Config {
	cfg := &helpersConfig{
		WriterConfig: NewWriterConfig(operatorID, helpersTestType.String()),
		Time:         NewTimeParser(),
		Severity:     NewSeverityConfig(),
		Scope:        NewScopeNameParser(),
	}
	return cfg
}

// This function is impelmented for compatibility with operator.Factory but is not meant to be used directly
func (f helpersFactory) CreateOperator(_ component.Config, _ component.TelemetrySettings) (operator.Operator, error) {
	panic("not implemented")
}
