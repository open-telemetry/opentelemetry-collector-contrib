// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Endpoint = "http://example.com"
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				Endpoint:            "https://example.com",
				AccessToken:         "abcd1234",
				NumWorkers:          3,
				MaxConnections:      45,
				LogDetailedResponse: true,
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: false,
				},
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 10 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueBatchConfig{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
					Sizer:        exporterhelper.RequestSizerTypeRequests,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	invalid := Config{
		AccessToken:    "abcd1234",
		NumWorkers:     3,
		MaxConnections: 45,
	}
	noEndpointErr := invalid.Validate()
	require.Error(t, noEndpointErr)

	invalid = Config{
		Endpoint:       ":123:456",
		AccessToken:    "abcd1234",
		NumWorkers:     3,
		MaxConnections: 45,
	}
	invalidURLErr := invalid.Validate()
	require.Error(t, invalidURLErr)

	invalid = Config{
		Endpoint:    "http://localhost",
		Compression: "nosuchcompression",
	}
	assert.Error(t, invalid.Validate())

	invalid = Config{
		Endpoint: "abcd1234",
		QueueSettings: exporterhelper.QueueBatchConfig{
			Enabled:   true,
			QueueSize: -1,
		},
	}

	require.Error(t, xconfmap.Validate(invalid))
}
