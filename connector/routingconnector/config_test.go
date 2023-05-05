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

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
				DefaultPipelines: []string{"traces/otlp-all"},
				ErrorMode:        ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
						Pipelines: []string{"traces/jaeger-acme", "traces/otlp-acme"},
					},
					{
						Statement: `route() where resource.attributes["X-Tenant"] == "globex"`,
						Pipelines: []string{"traces/otlp-globex"},
					},
				},
			},
		},
		{
			configPath: "config_metrics.yaml",
			id:         component.NewIDWithName(typeStr, ""),
			expected: &Config{
				DefaultPipelines: []string{"metrics/otlp-all"},
				ErrorMode:        ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
						Pipelines: []string{"metrics/jaeger-acme", "metrics/otlp-acme"},
					},
					{
						Statement: `route() where resource.attributes["X-Tenant"] == "globex"`,
						Pipelines: []string{"metrics/otlp-globex"},
					},
				},
			},
		},
		{
			configPath: "config_logs.yaml",
			id:         component.NewIDWithName(typeStr, ""),
			expected: &Config{
				DefaultPipelines: []string{"logs/otlp-all"},
				ErrorMode:        ottl.PropagateError,
				Table: []RoutingTableItem{
					{
						Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
						Pipelines: []string{"logs/jaeger-acme", "logs/otlp-acme"},
					},
					{
						Statement: `route() where resource.attributes["X-Tenant"] == "globex"`,
						Pipelines: []string{"logs/otlp-globex"},
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
						Pipelines: []string{"traces/otlp"},
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
						Statement: `route() where resource.attributes["attr"] == "acme"`,
					},
				},
			},
			error: "invalid route : no pipelines defined for the route",
		},
		{
			name: "no routes provided",
			config: &Config{
				DefaultPipelines: []string{"traces/default"},
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
