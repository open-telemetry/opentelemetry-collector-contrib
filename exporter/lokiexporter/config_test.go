// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/metadata"
)

func TestLoadConfigNewExporter(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				ClientConfig: confighttp.ClientConfig{
					Headers: map[string]configopaque.String{
						"X-Custom-Header": "loki_rocks",
					},
					Endpoint: "https://loki:3100/loki/api/v1/push",
					TLSSetting: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile:   "/var/lib/mycert.pem",
							CertFile: "certfile",
							KeyFile:  "keyfile",
						},
						Insecure: true,
					},
					ReadBufferSize:  123,
					WriteBufferSize: 345,
					Timeout:         time.Second * 10,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueConfig{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				DefaultLabelsEnabled: map[string]bool{
					"exporter": false,
					"job":      true,
					"instance": true,
					"level":    false,
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

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc string
		cfg  *Config
		err  error
	}{
		{
			desc: "QueueSettings are invalid",
			cfg:  &Config{QueueSettings: exporterhelper.QueueConfig{QueueSize: -1, Enabled: true}},
			err:  fmt.Errorf("queue settings has invalid configuration"),
		},
		{
			desc: "Endpoint is invalid",
			cfg:  &Config{},
			err:  fmt.Errorf("\"endpoint\" must be a valid URL"),
		},
		{
			desc: "Config is valid",
			cfg: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "https://loki.example.com",
				},
			},
			err: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.err != nil {
				assert.ErrorContains(t, err, tc.err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
