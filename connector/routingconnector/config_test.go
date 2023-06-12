// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

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
				DefaultPipelines: []component.ID{
					component.NewIDWithName(component.DataTypeTraces, "otlp-all"),
				},
				ErrorMode: ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where attributes["X-Tenant"] == "acme"`,
						Pipelines: []component.ID{
							component.NewIDWithName(component.DataTypeTraces, "jaeger-acme"),
							component.NewIDWithName(component.DataTypeTraces, "otlp-acme"),
						},
					},
					{
						Statement: `route() where attributes["X-Tenant"] == "globex"`,
						Pipelines: []component.ID{
							component.NewIDWithName(component.DataTypeTraces, "otlp-globex"),
						},
					},
				},
			},
		},
		{
			configPath: "config_metrics.yaml",
			id:         component.NewIDWithName(typeStr, ""),
			expected: &Config{
				DefaultPipelines: []component.ID{
					component.NewIDWithName(component.DataTypeMetrics, "otlp-all"),
				},
				ErrorMode: ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where attributes["X-Tenant"] == "acme"`,
						Pipelines: []component.ID{
							component.NewIDWithName(component.DataTypeMetrics, "jaeger-acme"),
							component.NewIDWithName(component.DataTypeMetrics, "otlp-acme"),
						},
					},
					{
						Statement: `route() where attributes["X-Tenant"] == "globex"`,
						Pipelines: []component.ID{
							component.NewIDWithName(component.DataTypeMetrics, "otlp-globex"),
						},
					},
				},
			},
		},
		{
			configPath: "config_logs.yaml",
			id:         component.NewIDWithName(typeStr, ""),
			expected: &Config{
				DefaultPipelines: []component.ID{
					component.NewIDWithName(component.DataTypeLogs, "otlp-all"),
				},
				ErrorMode: ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where attributes["X-Tenant"] == "acme"`,
						Pipelines: []component.ID{
							component.NewIDWithName(component.DataTypeLogs, "jaeger-acme"),
							component.NewIDWithName(component.DataTypeLogs, "otlp-acme"),
						},
					},
					{
						Statement: `route() where attributes["X-Tenant"] == "globex"`,
						Pipelines: []component.ID{
							component.NewIDWithName(component.DataTypeLogs, "otlp-globex"),
						},
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
			name: "no statement provided",
			config: &Config{
				Table: []RoutingTableItem{
					{
						Pipelines: []component.ID{
							component.NewIDWithName(component.DataTypeTraces, "otlp"),
						},
					},
				},
			},
			error: "invalid (empty) route : no statement provided",
		},
		{
			name: "no pipeline provided",
			config: &Config{
				Table: []RoutingTableItem{
					{
						Statement: `route() where attributes["attr"] == "acme"`,
					},
				},
			},
			error: "invalid route: no pipelines defined for the route",
		},
		{
			name: "no routes provided",
			config: &Config{
				DefaultPipelines: []component.ID{
					component.NewIDWithName(component.DataTypeTraces, "default"),
				},
			},
			error: "invalid routing table: the routing table is empty",
		},
		{
			name:   "empty config",
			config: &Config{},
			error:  "invalid routing table: the routing table is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualError(t, component.ValidateConfig(tt.config), tt.error)
		})
	}
}
