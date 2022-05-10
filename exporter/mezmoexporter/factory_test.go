// Copyright  The OpenTelemetry Authors
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

package mezmoexporter

import (
	"context"
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

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, config.Type(typeStr))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	qs := exporterhelper.NewDefaultQueueSettings()
	qs.Enabled = false

	assert.Equal(t, cfg, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		IngestURL:        defaultIngestURL,
		IngestKey:        "",

		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: 5 * time.Second,
		},
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
		QueueSettings: qs,
	})
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.IngestURL = "https://example.com:8088/services/collector"
	cfg.IngestKey = "1234-1234"

	params := componenttest.NewNopExporterCreateSettings()
	_, err := createLogsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateLogsExporterNoConfig(t *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	_, err := createLogsExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateLogsExporterInvalidEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.IngestURL = "urn:something:12345"
	params := componenttest.NewNopExporterCreateSettings()
	_, err := createLogsExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.IngestURL = "https://example.com:8088/services/collector"
	cfg.IngestKey = "1234-1234"
	params := componenttest.NewNopExporterCreateSettings()
	exp, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	cfg.IngestURL = "https://example.com"
	cfg.IngestKey = "testToken"
	exp, err = factory.CreateLogsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}
