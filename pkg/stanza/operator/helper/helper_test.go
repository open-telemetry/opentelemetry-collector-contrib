// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package helper

import (
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const helpersTestType = "helpers_test"

func init() {
	operator.Register(helpersTestType, func() operator.Builder { return newHelpersConfig() })
}

type helpersConfig struct {
	BasicConfig `mapstructure:",squash"`
	Writer      WriterConfig    `mapstructure:"writer"`
	Time        TimeParser      `mapstructure:"time"`
	Severity    SeverityConfig  `mapstructure:"severity"`
	Scope       ScopeNameParser `mapstructure:"scope"`
	Size        ByteSize        `mapstructure:"size"`
}

func newHelpersConfig() *helpersConfig {
	return &helpersConfig{
		BasicConfig: NewBasicConfig(helpersTestType, helpersTestType),
		Writer:      NewWriterConfig(helpersTestType, helpersTestType),
		Time:        NewTimeParser(),
		Severity:    NewSeverityConfig(),
		Scope:       NewScopeNameParser(),
	}
}

// This function is impelmented for compatibility with operatortest
// but is not meant to be used directly
func (h *helpersConfig) Build(*zap.SugaredLogger) (operator.Operator, error) {
	panic("not impelemented")
}
