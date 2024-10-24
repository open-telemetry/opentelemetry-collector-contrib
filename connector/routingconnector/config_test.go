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
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestLoadConfig(t *testing.T) {
	testcases := []struct {
		configPath string
		id         component.ID
		expected   component.Config
	}{
		{
			configPath: filepath.Join("testdata", "config", "traces.yaml"),
			id:         component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				DefaultPipelines: []pipeline.ID{
					pipeline.NewIDWithName(pipeline.SignalTraces, "otlp-all"),
				},
				ErrorMode: ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where attributes["X-Tenant"] == "acme"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalTraces, "jaeger-acme"),
							pipeline.NewIDWithName(pipeline.SignalTraces, "otlp-acme"),
						},
					},
					{
						Statement: `route() where attributes["X-Tenant"] == "globex"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalTraces, "otlp-globex"),
						},
					},
				},
			},
		},
		{
			configPath: filepath.Join("testdata", "config", "metrics.yaml"),
			id:         component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				DefaultPipelines: []pipeline.ID{
					pipeline.NewIDWithName(pipeline.SignalMetrics, "otlp-all"),
				},
				ErrorMode: ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where attributes["X-Tenant"] == "acme"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalMetrics, "jaeger-acme"),
							pipeline.NewIDWithName(pipeline.SignalMetrics, "otlp-acme"),
						},
					},
					{
						Statement: `route() where attributes["X-Tenant"] == "globex"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalMetrics, "otlp-globex"),
						},
					},
				},
			},
		},
		{
			configPath: filepath.Join("testdata", "config", "logs.yaml"),
			id:         component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				DefaultPipelines: []pipeline.ID{
					pipeline.NewIDWithName(pipeline.SignalLogs, "otlp-all"),
				},
				ErrorMode: ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where attributes["X-Tenant"] == "acme"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalLogs, "jaeger-acme"),
							pipeline.NewIDWithName(pipeline.SignalLogs, "otlp-acme"),
						},
					},
					{
						Statement: `route() where attributes["X-Tenant"] == "globex"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalLogs, "otlp-globex"),
						},
					},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.configPath, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(tt.configPath)
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

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
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalTraces, "otlp"),
						},
					},
				},
			},
			error: "invalid route: no condition or statement provided",
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
			error: "invalid route: no pipelines defined",
		},
		{
			name: "no routes provided",
			config: &Config{
				DefaultPipelines: []pipeline.ID{
					pipeline.NewIDWithName(pipeline.SignalTraces, "default"),
				},
			},
			error: "invalid routing table: the routing table is empty",
		},
		{
			name:   "empty config",
			config: &Config{},
			error:  "invalid routing table: the routing table is empty",
		},
		{
			name: "condition provided",
			config: &Config{
				Table: []RoutingTableItem{
					{
						Condition: `attributes["attr"] == "acme"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalTraces, "otlp"),
						},
					},
				},
			},
		},
		{
			name: "statement provided",
			config: &Config{
				Table: []RoutingTableItem{
					{
						Statement: `route() where attributes["attr"] == "acme"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalTraces, "otlp"),
						},
					},
				},
			},
		},
		{
			name: "both condition and statement provided",
			config: &Config{
				Table: []RoutingTableItem{
					{
						Condition: `attributes["attr"] == "acme"`,
						Statement: `route() where attributes["attr"] == "acme"`,
						Pipelines: []pipeline.ID{
							pipeline.NewIDWithName(pipeline.SignalTraces, "otlp"),
						},
					},
				},
			},
			error: "invalid route: both condition and statement provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.error == "" {
				assert.NoError(t, component.ValidateConfig(tt.config))
			} else {
				assert.EqualError(t, component.ValidateConfig(tt.config), tt.error)
			}
		})
	}
}
