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
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: func() *Config {
				c := createDefaultConfig().(*Config)
				c.URLs = []string{"nats://localhost:4222", "nats://localhost:4223"}
				c.Name = "otelcol"
				c.Connection = ConnectionConfig{Timeout: 5 * time.Second, MaxReconnects: -1, ReconnectWait: time.Second}
				c.Auth.Token = configoptional.Some(TokenAuth{Token: "s3cr3t"})
				c.Metrics.Encoding = "otlp_json"
				return c
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "jetstream"),
			expected: func() *Config {
				c := createDefaultConfig().(*Config)
				c.URLs = []string{"nats://localhost:4222"}
				c.JetStream = configoptional.Some(JetStreamConfig{PublishTimeout: 10 * time.Second})
				return c
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := createDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadConfigFailed(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id            component.ID
		errorContains string
	}{
		{component.NewIDWithName(metadata.Type, "auth_conflict"), "only one authentication method"},
		{component.NewIDWithName(metadata.Type, "no_urls"), "at least one url"},
		{component.NewIDWithName(metadata.Type, "nkey_no_seed"), "seed_file is required"},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := createDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.ErrorContains(t, xconfmap.Validate(cfg), tt.errorContains)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(*Config)
		errorContains string
	}{
		{
			name:   "valid",
			mutate: func(c *Config) { c.URLs = []string{"nats://localhost:4222"} },
		},
		{
			name:          "no urls",
			mutate:        func(*Config) {},
			errorContains: "at least one url",
		},
		{
			name: "conflicting auth",
			mutate: func(c *Config) {
				c.URLs = []string{"nats://localhost:4222"}
				c.Auth.Token = configoptional.Some(TokenAuth{Token: "t"})
				c.Auth.UserPassword = configoptional.Some(UserPasswordAuth{Username: "u", Password: "p"})
			},
			errorContains: "only one authentication method",
		},
		{
			name: "nkey without seed",
			mutate: func(c *Config) {
				c.URLs = []string{"nats://localhost:4222"}
				c.Auth.NKey = configoptional.Some(NKeyAuth{})
			},
			errorContains: "seed_file is required",
		},
		{
			name: "credentials without file",
			mutate: func(c *Config) {
				c.URLs = []string{"nats://localhost:4222"}
				c.Auth.Credentials = configoptional.Some(CredentialsAuth{})
			},
			errorContains: "file is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.errorContains == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.errorContains)
		})
	}
}
