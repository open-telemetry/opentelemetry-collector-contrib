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

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name   string
		config config.Processor
		error  string
	}{
		{
			name: "both expression and value specified",
			config: &Config{
				FromAttribute:   "attr",
				AttributeSource: resourceAttributeSource,
				Table: []RoutingTableItem{
					{
						Exporters:  []string{"otlp"},
						Value:      "acme",
						Expression: `route() where resource.attributes["attr"] == "acme"`,
					},
				},
			},
			error: "invalid route: both expression (route() where resource.attributes[\"attr\"] == \"acme\") and value (acme) provided",
		},
		{
			name: "neither expression or value provided",
			config: &Config{
				FromAttribute:   "attr",
				AttributeSource: resourceAttributeSource,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
					},
				},
			},
			error: "invalid (empty) route : empty routing attribute provided",
		},
		{
			name: "drop routing attribute with context as routing attribute source",
			config: &Config{
				FromAttribute:                "attr",
				AttributeSource:              contextAttributeSource,
				DropRoutingResourceAttribute: true,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Value:     "test",
					},
				},
			},
			error: "using a different attribute source than 'attribute' and drop_resource_routing_attribute is set to true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualError(t, tt.config.Validate(), tt.error)
		})
	}
}

func TestRewriteLegacyConfigToOTTL(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   Config
	}{
		{
			name: "rewrite routing by resource attribute",
			config: Config{
				FromAttribute:   "attr",
				AttributeSource: resourceAttributeSource,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Value:     "acme",
					},
				},
			},
			want: Config{
				Table: []RoutingTableItem{
					{
						Exporters:  []string{"otlp"},
						Expression: `route() where resource.attributes["attr"] == "acme"`,
					},
				},
			},
		},
		{
			name: "rewrite routing by resource attribute multiple entries",
			config: Config{
				FromAttribute:   "attr",
				AttributeSource: resourceAttributeSource,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Value:     "acme",
					},
					{
						Exporters: []string{"otlp/2"},
						Value:     "ecorp",
					},
				},
			},
			want: Config{
				Table: []RoutingTableItem{
					{
						Exporters:  []string{"otlp"},
						Expression: `route() where resource.attributes["attr"] == "acme"`,
					},
					{
						Exporters:  []string{"otlp/2"},
						Expression: `route() where resource.attributes["attr"] == "ecorp"`,
					},
				},
			},
		},
		{
			name: "rewrite routing by resource attribute with dropping routing key",
			config: Config{
				FromAttribute:                "attr",
				AttributeSource:              resourceAttributeSource,
				DropRoutingResourceAttribute: true,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Value:     "acme",
					},
				},
			},
			want: Config{
				Table: []RoutingTableItem{
					{
						Exporters:  []string{"otlp"},
						Expression: `delete_key(resource.attributes, "attr") where resource.attributes["attr"] == "acme"`,
					},
				},
			},
		},
		{
			name: "rewrite routing with context as attribute source",
			config: Config{
				FromAttribute:   "attr",
				AttributeSource: contextAttributeSource,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Value:     "acme",
					},
				},
			},
			want: Config{
				FromAttribute:   "attr",
				AttributeSource: contextAttributeSource,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Value:     "acme",
					},
				},
			},
		},
		{
			name: "rewrite routing by resource attribute with mixed routing entries",
			config: Config{
				FromAttribute:   "attr",
				AttributeSource: resourceAttributeSource,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Value:     "acme",
					},
					{
						Exporters:  []string{"otlp/2"},
						Expression: `route() where resource.attributes["attr"] == "ecorp"`,
					},
				},
			},
			want: Config{
				Table: []RoutingTableItem{
					{
						Exporters:  []string{"otlp"},
						Expression: `route() where resource.attributes["attr"] == "acme"`,
					},
					{
						Exporters:  []string{"otlp/2"},
						Expression: `route() where resource.attributes["attr"] == "ecorp"`,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, *rewriteRoutingEntriesToOTTL(&tt.config))
		})
	}
}
