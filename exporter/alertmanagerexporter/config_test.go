// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter

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
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	// Endpoint doesn't have a default value so set it directly.
	defaultCfg := createDefaultConfig().(*Config)

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
				GeneratorURL:      "opentelemetry-collector",
				DefaultSeverity:   "info",
				SeverityAttribute: "foo",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 10 * time.Second,
				},
				BackoffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Headers: map[string]configopaque.String{
						"can you have a . here?": "F0000000-0000-0000-0000-000000000000",
						"header1":                "234",
						"another":                "somevalue",
					},
					Endpoint: "a.new.alertmanager.target:9093",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "/var/lib/mycert.pem",
						},
					},
					ReadBufferSize:  0,
					WriteBufferSize: 524288,
					Timeout:         time.Second * 10,
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "NoEndpoint",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.HTTPClientSettings.Endpoint = ""
				return cfg
			}(),
			wantErr: "endpoint must be non-empty",
		},
		{
			name: "NoSeverity",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.DefaultSeverity = ""
				return cfg
			}(),
			wantErr: "severity must be non-empty",
		},
		{
			name:    "Success",
			cfg:     createDefaultConfig().(*Config),
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
