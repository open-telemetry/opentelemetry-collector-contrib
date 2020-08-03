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

	e0 := cfg.Exporters["datadog"].(*Config)
	err = e0.Sanitize()

	require.NoError(t, err)
	assert.Equal(t, e0, &Config{
		ExporterSettings: configmodels.ExporterSettings{
			NameVal: "datadog",
			TypeVal: "datadog",
		},
		APIKey:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Site:            DefaultSite,
		MetricsEndpoint: "https://api.datadoghq.com",
	})

	e1 := cfg.Exporters["datadog/2"].(*Config)
	err = e1.Sanitize()

	require.NoError(t, err)
	assert.Equal(t, e1,
		&Config{
			ExporterSettings: configmodels.ExporterSettings{
				NameVal: "datadog/2",
				TypeVal: "datadog",
			},
			APIKey:          "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Site:            "datadoghq.eu",
			MetricsEndpoint: "https://api.datadoghq.eu",
		})
}

// TestOverrideMetricsEndpoint tests that the metrics endpoint is overridden
// correctly when set manually.
func TestOverrideMetricsEndpoint(t *testing.T) {

	const DebugEndpoint string = "http://localhost:8080"

	cfg := &Config{
		APIKey:          "notnull",
		Site:            DefaultSite,
		MetricsEndpoint: DebugEndpoint,
	}

	err := cfg.Sanitize()
	require.NoError(t, err)
	assert.Equal(t, cfg.MetricsEndpoint, DebugEndpoint)
}

// TestUnsetAPIKey tests that the config sanitizing throws an error
// when the API key has not been set
func TestUnsetAPIKey(t *testing.T) {

	cfg := &Config{}
	err := cfg.Sanitize()

	require.Error(t, err)
}
