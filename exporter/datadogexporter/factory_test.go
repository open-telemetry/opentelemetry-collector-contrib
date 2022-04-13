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
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/servicetest"
	"go.uber.org/zap"

	ddconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		Metrics: ddconfig.MetricsConfig{
			DeltaTTL:      3600,
			SendMonotonic: true,
			Quantiles:     true,
			HistConfig: ddconfig.HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: ddconfig.SumConfig{
				CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
			},
		},

		Traces: ddconfig.TracesConfig{
			SampleRate:      1,
			IgnoreResources: []string{},
		},

		HostMetadata: ddconfig.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: ddconfig.HostnameSourceFirstResource,
		},

		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, cfg, "failed to create default config")

	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	apiConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")].(*ddconfig.Config)
	err = apiConfig.Sanitize(zap.NewNop())

	require.NoError(t, err)
	assert.Equal(t, config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "api")), apiConfig.ExporterSettings)
	assert.Equal(t, defaulttimeoutSettings(), apiConfig.TimeoutSettings)
	assert.Equal(t, exporterhelper.NewDefaultRetrySettings(), apiConfig.RetrySettings)
	assert.Equal(t, exporterhelper.NewDefaultQueueSettings(), apiConfig.QueueSettings)
	assert.Equal(t, ddconfig.TagsConfig{
		Hostname: "customhostname",
		Env:      "prod",
		Service:  "myservice",
		Version:  "myversion",
		Tags:     []string{"example:tag"},
	}, apiConfig.TagsConfig)
	assert.Equal(t, ddconfig.APIConfig{
		Key:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Site: "datadoghq.eu",
	}, apiConfig.API)
	assert.Equal(t, ddconfig.MetricsConfig{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "https://api.datadoghq.eu",
		},
		DeltaTTL:      3600,
		SendMonotonic: true,
		Quantiles:     true,
		HistConfig: ddconfig.HistogramConfig{
			Mode:         "distributions",
			SendCountSum: false,
		},
		SumConfig: ddconfig.SumConfig{
			CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
		},
	}, apiConfig.Metrics)
	assert.Equal(t, ddconfig.TracesConfig{
		SampleRate: 1,
		TCPAddr: confignet.TCPAddr{
			Endpoint: "https://trace.agent.datadoghq.eu",
		},
		IgnoreResources: []string{},
	}, apiConfig.Traces)
	assert.True(t, apiConfig.SendMetadata)
	assert.False(t, apiConfig.OnlyMetadata)
	assert.True(t, apiConfig.UseResourceMetadata)

	defaultConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "default")].(*ddconfig.Config)
	err = defaultConfig.Sanitize(zap.NewNop())

	require.NoError(t, err)
	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "default")),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		TagsConfig: ddconfig.TagsConfig{
			Hostname: "",
			Env:      "none",
			Service:  "",
			Version:  "",
		},

		API: ddconfig.APIConfig{
			Key:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site: "datadoghq.com",
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.com",
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
			Quantiles:     true,
			HistConfig: ddconfig.HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: ddconfig.SumConfig{
				CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
			},
		},

		Traces: ddconfig.TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			IgnoreResources: []string{},
		},

		HostMetadata: ddconfig.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: ddconfig.HostnameSourceFirstResource,
		},

		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, defaultConfig)

	invalidConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "invalid")].(*ddconfig.Config)
	err = invalidConfig.Sanitize(zap.NewNop())
	require.Error(t, err)

	hostMetadataConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "hostmetadata")].(*ddconfig.Config)
	err = hostMetadataConfig.Sanitize(zap.NewNop())
	require.NoError(t, err)

	assert.Equal(t, ddconfig.HostMetadataConfig{
		Enabled:        true,
		HostnameSource: ddconfig.HostnameSourceConfigOrSystem,
		Tags:           []string{"example:one"},
	}, hostMetadataConfig.HostMetadata)
}

func TestCreateAPIMetricsExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*ddconfig.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")],
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPITracesExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*ddconfig.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateTracesExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")],
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestOnlyMetadata(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	ctx := context.Background()
	cfg := &ddconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		API:          ddconfig.APIConfig{Key: "notnull"},
		Metrics:      ddconfig.MetricsConfig{TCPAddr: confignet.TCPAddr{Endpoint: server.URL}},
		Traces:       ddconfig.TracesConfig{TCPAddr: confignet.TCPAddr{Endpoint: server.URL}},
		OnlyMetadata: true,

		HostMetadata: ddconfig.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: ddconfig.HostnameSourceFirstResource,
		},
	}

	expTraces, err := factory.CreateTracesExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)

	expMetrics, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expMetrics)

	err = expTraces.Start(ctx, nil)
	assert.NoError(t, err)
	defer expTraces.Shutdown(ctx)

	err = expTraces.ConsumeTraces(ctx, testutils.TestTraces.Clone())
	require.NoError(t, err)

	body := <-server.MetadataChan
	var recvMetadata metadata.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")

}
