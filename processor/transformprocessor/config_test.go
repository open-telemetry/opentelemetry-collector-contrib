// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transformprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.ProcessorConfig
		errorMessage string
	}{
		{
			id: component.NewIDWithName(typeStr, ""),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(component.NewID(typeStr)),
				OTTLConfig: OTTLConfig{
					Traces: SignalConfig{
						Statements: []string{},
					},
					Metrics: SignalConfig{
						Statements: []string{},
					},
					Logs: SignalConfig{
						Statements: []string{},
					},
				},
				TraceStatements: []common.ContextStatements{
					{
						Context: "trace",
						Statements: []string{
							`set(name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				MetricStatements: []common.ContextStatements{
					{
						Context: "datapoint",
						Statements: []string{
							`set(metric.name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				LogStatements: []common.ContextStatements{
					{
						Context: "log",
						Statements: []string{
							`set(body, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "deprecated_format"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(component.NewIDWithName(typeStr, "")),
				OTTLConfig: OTTLConfig{
					Traces: SignalConfig{
						Statements: []string{
							`set(name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					Metrics: SignalConfig{
						Statements: []string{
							`set(metric.name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					Logs: SignalConfig{
						Statements: []string{
							`set(body, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
				},
				TraceStatements:  []common.ContextStatements{},
				MetricStatements: []common.ContextStatements{},
				LogStatements:    []common.ContextStatements{},
			},
		},
		{
			id:           component.NewIDWithName(typeStr, "using_both_formats"),
			errorMessage: "cannot use Traces, Metrics and/or Logs with TraceStatements, MetricStatements and/or LogStatements",
		},
		{
			id:           component.NewIDWithName(typeStr, "bad_syntax_trace"),
			errorMessage: "1:18: unexpected token \"where\" (expected \")\")",
		},
		{
			id:           component.NewIDWithName(typeStr, "unknown_function_trace"),
			errorMessage: "undefined function not_a_function",
		},
		{
			id:           component.NewIDWithName(typeStr, "bad_syntax_metric"),
			errorMessage: "1:18: unexpected token \"where\" (expected \")\")",
		},
		{
			id:           component.NewIDWithName(typeStr, "unknown_function_metric"),
			errorMessage: "undefined function not_a_function",
		},
		{
			id:           component.NewIDWithName(typeStr, "bad_syntax_log"),
			errorMessage: "1:18: unexpected token \"where\" (expected \")\")",
		},
		{
			id:           component.NewIDWithName(typeStr, "unknown_function_log"),
			errorMessage: "undefined function not_a_function",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			assert.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			assert.NoError(t, err)
			assert.NoError(t, component.UnmarshalProcessorConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, cfg.Validate(), tt.errorMessage)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func Test_UnknownContextID(t *testing.T) {
	id := component.NewIDWithName(typeStr, "unknown_context")

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	assert.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	assert.NoError(t, err)
	assert.Error(t, component.UnmarshalProcessorConfig(sub, cfg))
}
