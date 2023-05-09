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

package splunkhecexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopCreateSettings()
	_, err := createMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopCreateSettings()
	_, err := createTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopCreateSettings()
	_, err := createLogsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"
	params := exportertest.NewNopCreateSettings()
	exp, err := factory.CreateMetricsExporter(
		context.Background(), params,
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	cfg.Token = "testToken"
	cfg.HTTPClientSettings.Endpoint = "https://example.com"
	exp, err = factory.CreateMetricsExporter(
		context.Background(), params,
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestFactory_CreateMetricsExporter(t *testing.T) {
	config := &Config{
		Token: "testToken",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://example.com:8000",
		},
	}

	params := exportertest.NewNopCreateSettings()
	te, err := createMetricsExporter(context.Background(), params, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}
