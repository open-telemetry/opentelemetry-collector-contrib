// Copyright The OpenTelemetry Authors
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

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig().(*Config)
	defaultCfg.IngestURL = defaultIngestURL
	defaultCfg.IngestKey = "00000000000000000000000000000000"

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(typeStr, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(typeStr, "allsettings"),
			expected: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout: 5 * time.Second,
				},
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:             false,
					InitialInterval:     99 * time.Second,
					MaxInterval:         199 * time.Second,
					MaxElapsedTime:      299 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      false,
					NumConsumers: 7,
					QueueSize:    17,
				},
				IngestURL: "https://alternate.mezmo.com/otel/ingest/rest",
				IngestKey: "1234509876",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigInvalidEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.IngestURL = "urn:something:12345"
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_Path(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.IngestURL = "https://example.com:8088/ingest/rest"
	cfg.IngestKey = "1234-1234"
	assert.NoError(t, cfg.Validate())

	cfg.IngestURL = "https://example.com:8088/v1/ABC123"
	cfg.IngestKey = "1234-1234"
	assert.NoError(t, cfg.Validate())

	// Set values that don't have a valid default.
	cfg.IngestURL = "/nohost/path"
	cfg.IngestKey = "testToken"
	assert.Error(t, cfg.Validate())
}
