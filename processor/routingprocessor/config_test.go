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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestLoadConfig(t *testing.T) {
	testcases := []struct {
		configPath string
		id         component.ID
		expected   component.Config
	}{
		{
			configPath: "config_traces.yaml",
			id:         component.NewIDWithName(typeStr, ""),
			expected: &Config{
				DefaultExporters: []string{"otlp"},
				AttributeSource:  "context",
				FromAttribute:    "X-Tenant",
				ErrorMode:        ottl.PropagateError,
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
			id:         component.NewIDWithName(typeStr, ""),
			expected: &Config{
				DefaultExporters: []string{"logging/default"},
				AttributeSource:  "context",
				FromAttribute:    "X-Custom-Metrics-Header",
				ErrorMode:        ottl.PropagateError,
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
			id:         component.NewIDWithName(typeStr, ""),
			expected: &Config{
				DefaultExporters: []string{"logging/default"},
				AttributeSource:  "context",
				FromAttribute:    "X-Custom-Logs-Header",
				ErrorMode:        ottl.PropagateError,
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
			configPath: "config.yaml",
			id:         component.NewIDWithName(typeStr, ""),
			expected: &Config{
				DefaultExporters: []string{"jaeger"},
				AttributeSource:  resourceAttributeSource,
				FromAttribute:    "X-Tenant",
				ErrorMode:        ottl.IgnoreError,
				Table: []RoutingTableItem{
					{
						Value:     "acme",
						Exporters: []string{"otlp/traces"},
					},
				},
			},
		},
		{
			configPath: "config.yaml",
			id:         component.NewIDWithName(typeStr, "ottl"),
			expected: &Config{
				DefaultExporters: []string{"jaeger"},
				ErrorMode:        ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: "route() where resource.attributes[\"X-Tenant\"] == \"acme\"",
						Exporters: []string{"jaeger/acme"},
					},
					{
						Statement: "delete_key(resource.attributes, \"X-Tenant\") where IsMatch(resource.attributes[\"X-Tenant\"], \".*corp\")",
						Exporters: []string{"jaeger/ecorp"},
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

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name   string
		config component.Config
		error  string
	}{
		{
			name: "both statement and value specified",
			config: &Config{
				FromAttribute:   "attr",
				AttributeSource: resourceAttributeSource,
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Value:     "acme",
						Statement: `route() where resource.attributes["attr"] == "acme"`,
					},
				},
			},
			error: "invalid route: both statement (route() where resource.attributes[\"attr\"] == \"acme\") and value (acme) provided",
		},
		{
			name: "neither statement or value provided",
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
			assert.EqualError(t, component.ValidateConfig(tt.config), tt.error)
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
						Exporters: []string{"otlp"},
						Statement: `route() where resource.attributes["attr"] == "acme"`,
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
						Exporters: []string{"otlp"},
						Statement: `route() where resource.attributes["attr"] == "acme"`,
					},
					{
						Exporters: []string{"otlp/2"},
						Statement: `route() where resource.attributes["attr"] == "ecorp"`,
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
						Exporters: []string{"otlp"},
						Statement: `delete_key(resource.attributes, "attr") where resource.attributes["attr"] == "acme"`,
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
						Exporters: []string{"otlp/2"},
						Statement: `route() where resource.attributes["attr"] == "ecorp"`,
					},
				},
			},
			want: Config{
				Table: []RoutingTableItem{
					{
						Exporters: []string{"otlp"},
						Statement: `route() where resource.attributes["attr"] == "acme"`,
					},
					{
						Exporters: []string{"otlp/2"},
						Statement: `route() where resource.attributes["attr"] == "ecorp"`,
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
