// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	testcases := []struct {
		configPath string
		expected   config.Processor
	}{
		{
			configPath: "config_traces.yaml",
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				DefaultExporters:  []string{"otlp"},
				AttributeSource:   "context",
				FromAttribute:     "X-Tenant",
				Table: []RoutingTableItem{
					{
						Value:     "acme",
						Exporters: []string{"jaeger/acme", "otlp/acme"},
					},
					{
						Value:     "globex",
						Exporters: []string{"otlp/globex"},
					},
				},
			},
		},
		{
			configPath: "config_metrics.yaml",
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				DefaultExporters:  []string{"logging/default"},
				AttributeSource:   "context",
				FromAttribute:     "X-Custom-Metrics-Header",
				Table: []RoutingTableItem{
					{
						Value:     "acme",
						Exporters: []string{"logging/acme"},
					},
					{
						Value:     "globex",
						Exporters: []string{"logging/globex"},
					},
				},
			},
		},
		{
			configPath: "config_logs.yaml",
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				DefaultExporters:  []string{"logging/default"},
				AttributeSource:   "context",
				FromAttribute:     "X-Custom-Logs-Header",
				Table: []RoutingTableItem{
					{
						Value:     "acme",
						Exporters: []string{"logging/acme"},
					},
					{
						Value:     "globex",
						Exporters: []string{"logging/globex"},
					},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.configPath, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configPath))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(config.NewComponentIDWithName(typeStr, "").String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
