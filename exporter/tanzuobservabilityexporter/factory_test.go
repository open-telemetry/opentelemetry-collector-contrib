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

package tanzuobservabilityexporter

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, configtest.CheckConfigStruct(cfg))

	actual, ok := cfg.(*Config)
	require.True(t, ok, "invalid Config: %#v", cfg)
	assert.Equal(t, "http://localhost:30001", actual.Traces.Endpoint)
}

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[exporterType] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	actual, ok := cfg.Exporters[config.NewComponentID("tanzuobservability")]
	require.True(t, ok)
	expected := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("tanzuobservability")),
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:40001"},
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 2,
			QueueSize:    10,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         true,
			InitialInterval: 10 * time.Second,
			MaxInterval:     60 * time.Second,
			MaxElapsedTime:  10 * time.Minute,
		},
	}
	assert.Equal(t, expected, actual)
}

func TestCreateExporter(t *testing.T) {
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	params := componenttest.NewNopExporterCreateSettings()

	te, err := createTracesExporter(context.Background(), params, cfg)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
}

func TestCreateTraceExporterNilConfigError(t *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	_, err := createTracesExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateTraceExporterInvalidEndpointError(t *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Traces.Endpoint = "http:#$%^&#$%&#"
	_, err := createTracesExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateTraceExporterMissingPortError(t *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Traces.Endpoint = "http://localhost"
	_, err := createTracesExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateTraceExporterInvalidPortError(t *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Traces.Endpoint = "http://localhost:c42a"
	_, err := createTracesExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}
