// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerobserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				Endpoint:              "unix:///var/run/docker.sock",
				CacheSyncInterval:     5 * time.Minute,
				Timeout:               20 * time.Second,
				ExcludedImages:        []string{"excluded", "image"},
				UseHostnameIfPresent:  true,
				UseHostBindings:       true,
				IgnoreNonHostBindings: true,
				DockerAPIVersion:      1.22,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := loadConfig(t, tt.id)
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := &Config{}
	assert.Equal(t, "endpoint must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint"}
	assert.Equal(t, "api_version must be at least 1.22", component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: 1.22}
	assert.Equal(t, "timeout must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: 1.22, Timeout: 5 * time.Minute}
	assert.Equal(t, "cache_sync_interval must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: 1.22, Timeout: 5 * time.Minute, CacheSyncInterval: 5 * time.Minute}
	assert.Nil(t, component.ValidateConfig(cfg))
}

func loadConfig(t testing.TB, id component.ID) *Config {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	return cfg.(*Config)
}
