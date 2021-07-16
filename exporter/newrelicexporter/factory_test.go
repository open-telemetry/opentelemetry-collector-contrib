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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcheck"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, configcheck.ValidateConfig(cfg))

	nrCfg, ok := cfg.(*Config)
	require.True(t, ok, "invalid Config: %#v", cfg)
	assert.Equal(t, nrCfg.CommonConfig.TimeoutSettings.Timeout, time.Second*5)
}

func TestCreateExporterWithAPIKey(t *testing.T) {
	cfg := createDefaultConfig()
	nrConfig := cfg.(*Config)
	nrConfig.MetricsConfig.APIKey = "a1b2c3d4"
	nrConfig.TracesConfig.APIKey = "a1b2c3d4"
	nrConfig.LogsConfig.APIKey = "a1b2c3d4"
	params := componenttest.NewNopExporterCreateSettings()

	te, err := createTracesExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := createMetricsExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")

	le, err := createLogsExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, le, "failed to create logs exporter")
}

func TestCreateExporterWithAPIKeyHeader(t *testing.T) {
	cfg := createDefaultConfig()
	nrConfig := cfg.(*Config)
	nrConfig.MetricsConfig.APIKeyHeader = "api-key"
	nrConfig.TracesConfig.APIKeyHeader = "api-key"
	nrConfig.LogsConfig.APIKeyHeader = "api-key"
	params := componenttest.NewNopExporterCreateSettings()

	te, err := createTracesExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := createMetricsExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")

	le, err := createLogsExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, le, "failed to create logs exporter")
}

func TestCreateExporterWithAPIKeyAndAPIKeyHeader(t *testing.T) {
	cfg := createDefaultConfig()
	nrConfig := cfg.(*Config)
	nrConfig.MetricsConfig.APIKey = "a1b2c3d4"
	nrConfig.TracesConfig.APIKey = "a1b2c3d4"
	nrConfig.LogsConfig.APIKey = "a1b2c3d4"
	nrConfig.MetricsConfig.APIKeyHeader = "api-key"
	nrConfig.TracesConfig.APIKeyHeader = "api-key"
	nrConfig.LogsConfig.APIKeyHeader = "api-key"
	params := componenttest.NewNopExporterCreateSettings()

	te, err := createTracesExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := createMetricsExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")

	le, err := createLogsExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, le, "failed to create logs exporter")
}

func TestCreateExporterErrorWithoutAPIKeyOrAPIKeyHeader(t *testing.T) {
	cfg := createDefaultConfig()
	nrConfig := cfg.(*Config)
	params := componenttest.NewNopExporterCreateSettings()

	te, err := createTracesExporter(context.Background(), params, nrConfig)
	assert.NotNil(t, err)
	assert.Nil(t, te)

	me, err := createMetricsExporter(context.Background(), params, nrConfig)
	assert.NotNil(t, err)
	assert.Nil(t, me)

	le, err := createLogsExporter(context.Background(), params, nrConfig)
	assert.NotNil(t, err)
	assert.Nil(t, le)
}
func TestCreateTracesExporterError(t *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	_, err := createTracesExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateMetricsExporterError(t *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	_, err := createMetricsExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateLogsExporterError(t *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	_, err := createLogsExporter(context.Background(), params, nil)
	assert.Error(t, err)
}
