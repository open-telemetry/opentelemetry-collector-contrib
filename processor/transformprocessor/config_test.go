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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlconfig"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           config.ComponentID
		expected     config.Processor
		errorMessage string
	}{
		{
			id: config.NewComponentIDWithName(typeStr, ""),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Config: ottlconfig.Config{
					Traces: ottlconfig.SignalConfig{
						Queries: []string{
							`set(name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, "http.method", "http.path")`,
						},
					},
					Metrics: ottlconfig.SignalConfig{
						Queries: []string{
							`set(metric.name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, "http.method", "http.path")`,
						},
					},
					Logs: ottlconfig.SignalConfig{
						Queries: []string{
							`set(body, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, "http.method", "http.path")`,
						},
					},
				},
			},
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "bad_syntax_trace"),
			errorMessage: "1:18: unexpected token \"where\" (expected \")\")",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "unknown_function_trace"),
			errorMessage: "undefined function not_a_function",
		},

		{
			id:           config.NewComponentIDWithName(typeStr, "bad_syntax_metric"),
			errorMessage: "1:18: unexpected token \"where\" (expected \")\")",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "unknown_function_metric"),
			errorMessage: "undefined function not_a_function",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "bad_syntax_log"),
			errorMessage: "1:18: unexpected token \"where\" (expected \")\")",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "unknown_function_log"),
			errorMessage: "undefined function not_a_function",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, cfg.Validate(), tt.errorMessage)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
