// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(component.MustNewType("nats"), ""),
			expected: &Config{
				Endpoint: "nats://localhost:4222",
				// Signal subjects retain the factory defaults after merge.
				Logs:    SignalConfig{Subject: defaultLogsSubject},
				Metrics: SignalConfig{Subject: defaultMetricsSubject},
				Traces:  SignalConfig{Subject: defaultTracesSubject},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("nats"), "full"),
			expected: &Config{
				Endpoint: "nats://nats.example.com:4222",
				Pedantic: false,
				JetStream: &JetStreamConfig{
					Domain:         "hub",
					PublishTimeout: 5 * time.Second,
				},
				Logs:    SignalConfig{Subject: `"otel.logs"`, Marshaler: "otlp_proto"},
				Metrics: SignalConfig{Subject: `"otel.metrics"`, Marshaler: "otlp_json"},
				Traces:  SignalConfig{Subject: `"otel.spans"`, EncodingExtension: "otlp_encoding/nats"},
				Auth: AuthConfig{
					User: &UserConfig{Username: "otel", Password: "s3cret"},
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

			assert.NoError(t, confmap.Validate(cfg))
			// Only compare the fields exercised by testdata; TLS defaults are
			// populated by CreateDefaultConfig and not asserted here.
			got := cfg.(*Config)
			exp := tt.expected.(*Config)
			assert.Equal(t, exp.Endpoint, got.Endpoint)
			assert.Equal(t, exp.JetStream, got.JetStream)
			assert.Equal(t, exp.Logs, got.Logs)
			assert.Equal(t, exp.Metrics, got.Metrics)
			assert.Equal(t, exp.Traces, got.Traces)
			assert.Equal(t, exp.Auth, got.Auth)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "valid",
			cfg: &Config{
				Endpoint: "nats://localhost:4222",
				Logs:     SignalConfig{Marshaler: "otlp_proto"},
			},
		},
		{
			name: "unsupported marshaler",
			cfg: &Config{
				Logs: SignalConfig{Marshaler: "yaml"},
			},
			wantErr: "unsupported marshaler",
		},
		{
			name: "marshaler configured twice",
			cfg: &Config{
				Metrics: SignalConfig{Marshaler: "otlp_proto", EncodingExtension: "otlp_encoding"},
			},
			wantErr: "marshaler configured more than once",
		},
		{
			name: "incomplete user auth",
			cfg: &Config{
				Auth: AuthConfig{User: &UserConfig{Username: "otel"}},
			},
			wantErr: "incomplete username/password auth configuration",
		},
		{
			name: "multiple nkey auth",
			cfg: &Config{
				Auth: AuthConfig{
					Nkey:         &NkeyConfig{PublicKey: "k", Seed: []byte("s")},
					NkeyUserFile: &NkeyUserFileConfig{UserFilePath: "/creds"},
				},
			},
			wantErr: "NKey auth configured more than once",
		},
		{
			name: "negative jetstream timeout",
			cfg: &Config{
				JetStream: &JetStreamConfig{PublishTimeout: -1},
			},
			wantErr: "publish_timeout must not be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
