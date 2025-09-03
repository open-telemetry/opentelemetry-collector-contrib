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
		{
			name: "returns nil for empty config",
			cfg:  Config{},
			err:  nil,
		},
		{
			name: "returns error for invalid logs subject",
			cfg: Config{
				Logs: LogsConfig{
					Subject: "invalid",
				},
			},
			err: errors.New("failed to parse logs subject"),
		},
		{
			name: "returns error for built-in marshaler and encoding extension configured simultaneously",
			cfg: Config{
				Logs: LogsConfig{
					BuiltinMarshalerName:  "otlp_json",
					EncodingExtensionName: "otlp",
				},
			},
			err: errors.New("built-in marshaler and encoding extension configured simultaneously"),
		},
		{
			name: "returns error for unsupported built-in marshaler",
			cfg: Config{
				Logs: LogsConfig{
					BuiltinMarshalerName: "unsupported",
				},
			},
			err: errors.New("unsupported built-in marshaler"),
		},
		{
			name: "returns error for invalid encoding extension name",
			cfg: Config{
				Logs: LogsConfig{
					EncodingExtensionName: "",
				},
			},
			err: errors.New("failed to unmarshal encoding extension name"),
		},
		{
			name: "returns nil for complete token configuration",
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
			name: "returns error for incomplete username/password configuration",
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
			name: "returns error for multiple auth methods configured simultaneously",
			cfg: Config{
				Auth: AuthConfig{
					Token: &TokenConfig{
						Token: "token",
					},
					NKey: &NKeyConfig{
						PublicKey: "public_key",
						Seed:      "seed",
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
					Subject:              "\"logs\"",
					BuiltinMarshalerName: "otlp_json",
				},
				Metrics: MetricsConfig{
					Subject:              "\"metrics\"",
					BuiltinMarshalerName: "otlp_json",
				},
				Traces: TracesConfig{
					Subject:              "\"traces\"",
					BuiltinMarshalerName: "otlp_json",
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
