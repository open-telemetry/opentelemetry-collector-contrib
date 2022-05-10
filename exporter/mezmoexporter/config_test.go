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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadDefaultConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify the "default"/required values-only configuration
	e := cfg.Exporters[config.NewComponentID(typeStr)]

	// Our expected default configuration should use the defaultIngestURL
	defaultCfg := factory.CreateDefaultConfig().(*Config)
	defaultCfg.IngestURL = defaultIngestURL
	defaultCfg.IngestKey = "00000000000000000000000000000000"
	assert.Equal(t, defaultCfg, e)
}

func TestLoadAllSettingsConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify values from the config override the default configuration
	e := cfg.Exporters[config.NewComponentIDWithName(typeStr, "allsettings")]

	// Our expected default configuration should use the defaultIngestURL
	expectedCfg := Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "allsettings")),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: 5 * time.Second,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         false,
			InitialInterval: 99 * time.Second,
			MaxInterval:     199 * time.Second,
			MaxElapsedTime:  299 * time.Minute,
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      false,
			NumConsumers: 7,
			QueueSize:    17,
		},
		IngestURL: "https://alternate.logdna.com/log/ingest",
		IngestKey: "1234509876",
	}
	assert.Equal(t, &expectedCfg, e)
}
