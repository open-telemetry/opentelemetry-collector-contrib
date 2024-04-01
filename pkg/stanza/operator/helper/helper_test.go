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

func newHelpersFactory() operator.Factory {
	return operator.NewFactory(
		helpersTestType,
		func(operatorID string) component.Config {
			cfg := &helpersConfig{
				WriterConfig: NewWriterConfig(operatorID, helpersTestType.String()),
				Time:         NewTimeParser(),
				Severity:     NewSeverityConfig(),
				Scope:        NewScopeNameParser(),
			}
			return cfg
		},
		func(_ component.TelemetrySettings, _ component.Config) (operator.Operator, error) {
			panic("not implemented")
		},
	)
}
