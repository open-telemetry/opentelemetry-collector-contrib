// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.uber.org/multierr"
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
			name: "marshaler and encoder",
			cfg: Config{
				Logs: LogsConfig{
					Marshaler: "otlp_json",
					Encoder:   "otlp",
				},
			},
			err: errors.New("marshaler and encoder cannot be configured simultaneously"),
		},
		{
			name: "unsupported marshaler",
			cfg: Config{
				Logs: LogsConfig{
					Marshaler: "unsupported",
				},
			},
			err: fmt.Errorf("unsupported marshaler: %s", "unsupported"),
		},
		{
			name: "logs using log_body marshaler",
			cfg: Config{
				Logs: LogsConfig{
					Marshaler: "log_body",
				},
			},
			err: nil,
		},
		{
			name: "non-logs using log_body marshaler",
			cfg: Config{
				Metrics: MetricsConfig{
					Marshaler: "log_body",
				},
				Traces: TracesConfig{
					Marshaler: "log_body",
				},
			},
			err: multierr.Append(
				fmt.Errorf("unsupported marshaler for metrics: %s", "log_body"),
				fmt.Errorf("unsupported marshaler for traces: %s", "log_body"),
			),
		},
		{
			name: "incomplete token configuration",
			cfg: Config{
				Auth: AuthConfig{
					Token: &TokenConfig{},
				},
			},
			err: errors.New("incomplete token configuration"),
		},
		{
			name: "nkey and user_jwt/user_credentials",
			cfg: Config{
				Auth: AuthConfig{
					NKey: &NKeyConfig{
						PubKey: "pub_key",
						SigKey: "sig_key",
					},
					UserJWT: &UserJWTConfig{
						JWT:    "jwt",
						SigKey: "sig_key",
					},
				},
			},
			err: errors.New("nkey and user_jwt/user_credentials cannot be configured simultaneously"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.err == nil {
				assert.NoError(t, err, tt.name)
			} else {
				assert.Equal(t, tt.err, err, tt.name)
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
					Subject:   "logs",
					Marshaler: "otlp_json",
				},
				Metrics: MetricsConfig{
					Subject:   "metrics",
					Marshaler: "otlp_json",
				},
				Traces: TracesConfig{
					Subject:   "traces",
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
