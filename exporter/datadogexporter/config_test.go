// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadogexporter

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	apiConfig := cfg.Exporters["datadog/api"].(*Config)
	err = apiConfig.Sanitize()

	require.NoError(t, err)
	assert.Equal(t, apiConfig, &Config{
		ExporterSettings: configmodels.ExporterSettings{
			NameVal: "datadog/api",
			TypeVal: "datadog",
		},

		TagsConfig: TagsConfig{
			Hostname: "customhostname",
			Env:      "prod",
			Service:  "myservice",
			Version:  "myversion",
			Tags:     []string{"example:tag"},
		},

		API: APIConfig{
			Key:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site: "datadoghq.eu",
		},

		Metrics: MetricsConfig{
			Mode:      AgentlessMode,
			Namespace: "opentelemetry",

			DogStatsD: DogStatsDConfig{
				Endpoint:  "127.0.0.1:8125",
				Telemetry: true,
			},

			Agentless: AgentlessConfig{
				Endpoint: "https://api.datadoghq.eu",
			},
		},
	})

	dogstatsdConfig := cfg.Exporters["datadog/dogstatsd"].(*Config)
	err = dogstatsdConfig.Sanitize()

	require.NoError(t, err)
	assert.Equal(t, dogstatsdConfig, &Config{
		ExporterSettings: configmodels.ExporterSettings{
			NameVal: "datadog/dogstatsd",
			TypeVal: "datadog",
		},

		TagsConfig: TagsConfig{},
		API:        APIConfig{Site: "datadoghq.com"},

		Metrics: MetricsConfig{
			Mode: DogStatsDMode,

			DogStatsD: DogStatsDConfig{
				Endpoint:  "127.0.0.1:8125",
				Telemetry: true,
			},

			Agentless: AgentlessConfig{},
		},
	})

	invalidConfig := cfg.Exporters["datadog/invalid"].(*Config)
	err = invalidConfig.Sanitize()
	require.Error(t, err)

	invalidConfig2 := cfg.Exporters["datadog/agentless/invalid"].(*Config)
	err = invalidConfig2.Sanitize()
	require.Error(t, err)

}

// TestOverrideMetricsURL tests that the metrics URL is overridden
// correctly when set manually.
func TestOverrideMetricsURL(t *testing.T) {

	const DebugEndpoint string = "http://localhost:8080"

	cfg := Config{
		API: APIConfig{Key: "notnull", Site: DefaultSite},
		Metrics: MetricsConfig{
			Mode:      AgentlessMode,
			Agentless: AgentlessConfig{Endpoint: DebugEndpoint},
		},
	}

	err := cfg.Sanitize()
	require.NoError(t, err)
	assert.Equal(t, cfg.Metrics.Agentless.Endpoint, DebugEndpoint)
}
