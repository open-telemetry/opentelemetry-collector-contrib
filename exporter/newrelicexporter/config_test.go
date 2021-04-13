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

package newrelicexporter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[config.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters["newrelic"]
	defaultConfig := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, r0, defaultConfig)

	r1 := cfg.Exporters["newrelic/alt"].(*Config)
	assert.Equal(t, r1, &Config{
		ExporterSettings: &config.ExporterSettings{
			TypeVal: "newrelic",
			NameVal: "newrelic/alt",
		},
		CommonConfig: EndpointConfig{
			APIKey:  "a1b2c3d4",
			Timeout: time.Second * 30,
		},
		MetricsConfig: EndpointConfig{
			HostOverride: "alt.metrics.newrelic.com",
			insecure:     false,
		},
		TracesConfig: EndpointConfig{
			HostOverride: "alt.spans.newrelic.com",
			insecure:     false,
		},
		LogsConfig: EndpointConfig{
			HostOverride: "alt.logs.newrelic.com",
			insecure:     false,
		},
	})
}

func TestEndpointSpecificConfigTakesPrecedence(t *testing.T) {
	config := Config{
		CommonConfig: EndpointConfig{
			APIKey:       "commonapikey",
			APIKeyHeader: "commonapikeyheader",
			HostOverride: "commonhost",
			Timeout:      time.Second * 10,
		},
		TracesConfig: EndpointConfig{
			APIKey:       "tracesapikey",
			APIKeyHeader: "tracesapikeyheader",
			HostOverride: "traceshost",
			Timeout:      time.Second * 20,
		},
		MetricsConfig: EndpointConfig{
			APIKey:       "metricsapikey",
			APIKeyHeader: "metricsapikeyheader",
			HostOverride: "metricshost",
			Timeout:      time.Second * 30,
		},
		LogsConfig: EndpointConfig{
			APIKey:       "logsapikey",
			APIKeyHeader: "logsapikeyheader",
			HostOverride: "logshost",
			Timeout:      time.Second * 40,
		},
	}

	assert.Equal(t, config.TracesConfig, config.GetTracesConfig())
	assert.Equal(t, config.MetricsConfig, config.GetMetricsConfig())
	assert.Equal(t, config.LogsConfig, config.GetLogsConfig())
}

func TestEndpointSpecificConfigUsedWhenDefined(t *testing.T) {
	config := Config{
		CommonConfig: EndpointConfig{
			APIKey:       "commonapikey",
			APIKeyHeader: "commonapikeyheader",
			HostOverride: "commonhost",
			Timeout:      time.Second * 10,
		},
		TracesConfig: EndpointConfig{
			APIKey:       "tracesapikey",
			HostOverride: "traceshost",
			Timeout:      time.Second * 20,
		},
		MetricsConfig: EndpointConfig{
			APIKeyHeader: "metricsapikeyheader",
			HostOverride: "metricshost",
			Timeout:      time.Second * 30,
		},
		LogsConfig: EndpointConfig{
			APIKey:       "logsapikey",
			APIKeyHeader: "logsapikeyheader",
			HostOverride: "logshost",
		},
	}

	expectedTraceConfig := EndpointConfig{
		APIKey:       "tracesapikey",
		APIKeyHeader: "commonapikeyheader",
		HostOverride: "traceshost",
		Timeout:      time.Second * 20,
	}
	expectedMetricConfig := EndpointConfig{
		APIKey:       "commonapikey",
		APIKeyHeader: "metricsapikeyheader",
		HostOverride: "metricshost",
		Timeout:      time.Second * 30,
	}
	expectedLogConfig := EndpointConfig{
		APIKey:       "logsapikey",
		APIKeyHeader: "logsapikeyheader",
		HostOverride: "logshost",
		Timeout:      time.Second * 10,
	}

	assert.Equal(t, expectedTraceConfig, config.GetTracesConfig())
	assert.Equal(t, expectedMetricConfig, config.GetMetricsConfig())
	assert.Equal(t, expectedLogConfig, config.GetLogsConfig())
}

func TestCommonConfigValuesUsed(t *testing.T) {
	config := Config{
		CommonConfig: EndpointConfig{
			APIKey:       "commonapikey",
			APIKeyHeader: "commonapikeyheader",
			HostOverride: "commonhost",
			Timeout:      time.Second * 10,
		},
		TracesConfig: EndpointConfig{
			APIKey:       "",
			APIKeyHeader: "",
			HostOverride: "",
			Timeout:      0,
		},
		MetricsConfig: EndpointConfig{
			APIKey:       "",
			APIKeyHeader: "",
			HostOverride: "",
			Timeout:      0,
		},
		LogsConfig: EndpointConfig{
			APIKey:       "",
			APIKeyHeader: "",
			HostOverride: "",
			Timeout:      0,
		},
	}

	assert.Equal(t, config.CommonConfig, config.GetTracesConfig())
	assert.Equal(t, config.CommonConfig, config.GetMetricsConfig())
	assert.Equal(t, config.CommonConfig, config.GetLogsConfig())
}
