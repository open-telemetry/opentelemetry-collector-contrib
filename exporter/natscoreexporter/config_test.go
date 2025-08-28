// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/metadata"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		err  error
	}{
		{name: "empty config", cfg: Config{}, err: nil},
		{
			name: "invalid logs subject",
			cfg: Config{
				Logs: LogsConfig{
					Subject: "invalid",
				},
			},
			err: errors.New("failed to parse logs subject"),
		},
		{
			name: "marshaler and encoder configured simultaneously",
			cfg: Config{
				Logs: LogsConfig{
					Marshaler: "otlp_json",
					Encoder:   "otlp",
				},
			},
			err: errors.New("marshaler and encoder configured simultaneously"),
		},
		{
			name: "unsupported marshaler",
			cfg: Config{
				Logs: LogsConfig{
					Marshaler: "unsupported",
				},
			},
			err: errors.New("unsupported marshaler"),
		},
		{
			name: "complete token configuration",
			cfg: Config{
				Auth: AuthConfig{
					Token: &TokenConfig{
						Token: "token",
					},
				},
			},
			err: nil,
		},
		{
			name: "incomplete username/password configuration",
			cfg: Config{
				Auth: AuthConfig{
					UserInfo: &UserInfoConfig{
						User: "user",
					},
				},
			},
			err: errors.New("incomplete user_info configuration"),
		},
		{
			name: "multiple auth methods configured simultaneously",
			cfg: Config{
				Auth: AuthConfig{
					Token: &TokenConfig{
						Token: "token",
					},
					NKey: &NKeyConfig{
						PubKey: "pub_key",
						SigKey: "sig_key",
					},
				},
			},
			err: errors.New("multiple auth methods configured simultaneously"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.err == nil {
				assert.NoError(t, err, tt.name)
			} else {
				assert.ErrorContains(t, err, tt.err.Error())
			}
		})
	}
}

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
			expected: &Config{
				Endpoint: "nats://localhost:1234",
				TLS:      configtls.NewDefaultClientConfig(),
				Logs: LogsConfig{
					Subject:   "\"logs\"",
					Marshaler: "otlp_json",
				},
				Metrics: MetricsConfig{
					Subject:   "\"metrics\"",
					Marshaler: "otlp_json",
				},
				Traces: TracesConfig{
					Subject:   "\"traces\"",
					Marshaler: "otlp_json",
				},
				Auth: AuthConfig{
					Token: &TokenConfig{
						Token: "token",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
