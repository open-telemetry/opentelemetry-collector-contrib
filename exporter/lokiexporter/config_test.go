// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter

import (
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
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/metadata"
)

func TestLoadConfigNewExporter(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Headers = map[string]configopaque.String{
		"X-Custom-Header": "loki_rocks",
	}
	clientConfig.Endpoint = "https://loki:3100/loki/api/v1/push"
	clientConfig.TLSSetting = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "/var/lib/mycert.pem",
			CertFile: "certfile",
			KeyFile:  "keyfile",
		},
		Insecure: true,
	}
	clientConfig.ReadBufferSize = 123
	clientConfig.WriteBufferSize = 345
	clientConfig.Timeout = time.Second * 10
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
				ClientConfig: clientConfig,
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

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "https://loki.example.com"
	testCases := []struct {
		desc string
		cfg  *Config
		err  string
	}{
		{
			desc: "QueueSettings are invalid",
			cfg:  &Config{QueueSettings: exporterhelper.QueueBatchConfig{QueueSize: -1, Enabled: true}},
			err:  "queue settings has invalid configuration",
		},
		{
			desc: "Endpoint is invalid",
			cfg:  &Config{},
			err:  "\"endpoint\" must be a valid URL",
		},
		{
			desc: "Config is valid",
			cfg: &Config{
				ClientConfig: clientConfig,
			},
			err: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.err != "" {
				assert.ErrorContains(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
