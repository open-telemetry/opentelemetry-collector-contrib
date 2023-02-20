// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
