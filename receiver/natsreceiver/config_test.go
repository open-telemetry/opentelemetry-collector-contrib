// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/natsclient"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.NewIDWithName(component.MustNewType("nats"), ""),
			expected: &Config{
				ClientConfig: natsclient.ClientConfig{Endpoint: "nats://localhost:4222"},
				Logs:         SignalConfig{Subject: defaultLogsSubject},
				Metrics:      SignalConfig{Subject: defaultMetricsSubject},
				Traces:       SignalConfig{Subject: defaultTracesSubject},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("nats"), "core"),
			expected: &Config{
				ClientConfig: natsclient.ClientConfig{
					Endpoint: "nats://nats.example.com:4222",
					Pedantic: true,
					Auth: natsclient.AuthConfig{
						User: &natsclient.UserConfig{Username: "otel", Password: "s3cret"},
					},
				},
				Logs:    SignalConfig{Subject: "otel.logs.>", QueueGroup: "otel-collectors", Encoding: "otlp_proto"},
				Metrics: SignalConfig{Subject: "otel.metrics", QueueGroup: "otel-collectors", Encoding: "otlp_json"},
				Traces:  SignalConfig{Subject: "otel.spans", EncodingExtension: "otlp_encoding/nats"},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("nats"), "jetstream"),
			expected: &Config{
				ClientConfig: natsclient.ClientConfig{Endpoint: "nats://nats.example.com:4222"},
				JetStream: &JetStreamConfig{
					Domain:     "hub",
					AckWait:    30 * time.Second,
					MaxDeliver: 5,
				},
				Logs:    SignalConfig{Subject: "otel.logs.>", Stream: "OTEL_LOGS", Durable: "otel_logs_consumer"},
				Metrics: SignalConfig{Subject: defaultMetricsSubject},
				Traces:  SignalConfig{Subject: defaultTracesSubject},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			require.NoError(t, confmap.Validate(cfg))

			// TLS defaults are populated by CreateDefaultConfig and not asserted here.
			got := cfg.(*Config)
			assert.Equal(t, tt.expected.ClientConfig.Endpoint, got.ClientConfig.Endpoint)
			assert.Equal(t, tt.expected.ClientConfig.Pedantic, got.ClientConfig.Pedantic)
			assert.Equal(t, tt.expected.JetStream, got.JetStream)
			assert.Equal(t, tt.expected.Logs, got.Logs)
			assert.Equal(t, tt.expected.Metrics, got.Metrics)
			assert.Equal(t, tt.expected.Traces, got.Traces)
			assert.Equal(t, tt.expected.ClientConfig.Auth, got.ClientConfig.Auth)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name:   "valid default",
			mutate: func(*Config) {},
		},
		{
			name:    "empty subject",
			mutate:  func(c *Config) { c.Logs.Subject = "" },
			wantErr: "logs: subject must not be empty",
		},
		{
			name:    "unsupported encoding",
			mutate:  func(c *Config) { c.Logs.Encoding = "yaml" },
			wantErr: "unsupported encoding",
		},
		{
			name: "encoding configured twice",
			mutate: func(c *Config) {
				c.Metrics.Encoding = "otlp_proto"
				c.Metrics.EncodingExtension = "otlp_encoding"
			},
			wantErr: "encoding configured more than once",
		},
		{
			name:    "invalid encoding extension",
			mutate:  func(c *Config) { c.Traces.EncodingExtension = "not a/valid/id" },
			wantErr: "failed to parse encoding extension name",
		},
		{
			name:    "queue group with whitespace",
			mutate:  func(c *Config) { c.Traces.QueueGroup = "otel collectors" },
			wantErr: "queue_group must not contain whitespace",
		},
		{
			name: "queue group with jetstream",
			mutate: func(c *Config) {
				c.JetStream = &JetStreamConfig{}
				c.Logs.QueueGroup = "otel-collectors"
			},
			wantErr: "logs: queue_group is not supported with jetstream",
		},
		{
			name:    "durable without jetstream",
			mutate:  func(c *Config) { c.Metrics.Durable = "otel_metrics_consumer" },
			wantErr: "metrics: stream and durable require jetstream",
		},
		{
			name:    "negative ack_wait",
			mutate:  func(c *Config) { c.JetStream = &JetStreamConfig{AckWait: -1} },
			wantErr: "ack_wait must not be negative",
		},
		{
			name:    "negative max_deliver",
			mutate:  func(c *Config) { c.JetStream = &JetStreamConfig{MaxDeliver: -1} },
			wantErr: "max_deliver must not be negative",
		},
		{
			name:    "incomplete user auth",
			mutate:  func(c *Config) { c.ClientConfig.Auth.User = &natsclient.UserConfig{Username: "otel"} },
			wantErr: "incomplete username/password auth configuration",
		},
		{
			name: "multiple auth methods",
			mutate: func(c *Config) {
				c.ClientConfig.Auth.Token = &natsclient.TokenConfig{Token: "t"}
				c.ClientConfig.Auth.NkeyUserFile = &natsclient.NkeyUserFileConfig{UserFilePath: "/creds"}
			},
			wantErr: "more than one auth method configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			tt.mutate(cfg)
			err := confmap.Validate(cfg)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
