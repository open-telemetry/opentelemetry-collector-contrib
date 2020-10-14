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
	"go.opentelemetry.io/collector/config/confignet"
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
	assert.Equal(t, &Config{
		ExporterSettings: configmodels.ExporterSettings{
			NameVal: "datadog/api",
			TypeVal: typeStr,
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
			Namespace: "opentelemetry.",
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.eu",
			},
		},

		Traces: TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.eu",
			},
		},
	}, apiConfig)

	invalidConfig2 := cfg.Exporters["datadog/invalid"].(*Config)
	err = invalidConfig2.Sanitize()
	require.Error(t, err)

}

func TestTags(t *testing.T) {
	tc := TagsConfig{
		Hostname: "customhost",
		Env:      "customenv",
		Service:  "customservice",
		Version:  "customversion",
		Tags:     []string{"key1:val1", "key2:val2"},
	}

	assert.ElementsMatch(t,
		[]string{
			"host:customhost",
			"env:customenv",
			"service:customservice",
			"version:customversion",
			"key1:val1",
			"key2:val2",
		},
		tc.GetTags(true), // get host
	)
}

// TestOverrideMetricsURL tests that the metrics URL is overridden
// correctly when set manually.
func TestOverrideMetricsURL(t *testing.T) {

	const DebugEndpoint string = "http://localhost:8080"

	cfg := Config{
		API: APIConfig{Key: "notnull", Site: DefaultSite},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: DebugEndpoint,
			},
		},
	}

	err := cfg.Sanitize()
	require.NoError(t, err)
	assert.Equal(t, cfg.Metrics.Endpoint, DebugEndpoint)
}

func TestAPIKeyUnset(t *testing.T) {
	cfg := Config{}
	err := cfg.Sanitize()
	assert.Equal(t, err, errUnsetAPIKey)
}

func TestCensorAPIKey(t *testing.T) {
	cfg := APIConfig{
		Key: "ddog_32_characters_long_api_key1",
	}

	assert.Equal(
		t,
		"***************************_key1",
		cfg.GetCensoredKey(),
	)
}
