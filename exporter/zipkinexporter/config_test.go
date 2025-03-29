// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	// URL doesn't have a default value so set it directly.
	defaultCfg := createDefaultConfig().(*Config)
	defaultCfg.Endpoint = "http://some.location.org:9411/api/v2/spans"
	maxIdleConns := 50
	idleConnTimeout := 5 * time.Second

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
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
				ClientConfig: withDefaultHTTPClientConfig(func(config *confighttp.ClientConfig) {
					config.Endpoint = "https://somedest:1234/api/v2/spans"
					config.WriteBufferSize = 524288
					config.Timeout = 5 * time.Second
					config.TLSSetting = configtls.ClientConfig{
						InsecureSkipVerify: true,
					}
					config.MaxIdleConns = maxIdleConns
					config.IdleConnTimeout = idleConnTimeout
				}),
				Format:             "proto",
				DefaultServiceName: "test_name",
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

func withDefaultHTTPClientConfig(fns ...func(config *confighttp.ClientConfig)) confighttp.ClientConfig {
	cfg := confighttp.NewDefaultClientConfig()
	for _, fn := range fns {
		fn(&cfg)
	}
	return cfg
}
