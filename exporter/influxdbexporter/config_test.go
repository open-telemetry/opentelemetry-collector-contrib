// Copyright 2021, OpenTelemetry Authors
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

package influxdbexporter

import (
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

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	configDefault := cfg.Exporters[config.NewID(typeStr)]
	assert.Equal(t, configDefault, factory.CreateDefaultConfig())

	configWithSettings := cfg.Exporters[config.NewIDWithName(typeStr, "withsettings")].(*Config)
	assert.Equal(t, configWithSettings, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewIDWithName(typeStr, "withsettings")),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:8080",
			Timeout:  500 * time.Millisecond,
			Headers:  map[string]string{"User-Agent": "OpenTelemetry -> Influx"},
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 3,
			QueueSize:    10,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         true,
			InitialInterval: 1 * time.Second,
			MaxInterval:     3 * time.Second,
			MaxElapsedTime:  10 * time.Second,
		},
		Org:           "my-org",
		Bucket:        "my-bucket",
		Token:         "my-token",
		MetricsSchema: "telegraf-prometheus-v2",
	})
}
