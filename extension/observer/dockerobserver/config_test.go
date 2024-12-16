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
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

var version = "1.40"

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id            component.ID
		expected      component.Config
		expectedError string
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				Config: docker.Config{
					Endpoint:         "unix:///var/run/docker.sock",
					Timeout:          20 * time.Second,
					ExcludedImages:   []string{"excluded", "image"},
					DockerAPIVersion: version,
				},
				CacheSyncInterval:     5 * time.Minute,
				UseHostnameIfPresent:  true,
				UseHostBindings:       true,
				IgnoreNonHostBindings: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := loadConfig(t, tt.id)
			if tt.expectedError != "" {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.expectedError)
			} else {
				assert.NoError(t, component.ValidateConfig(cfg))
			}
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := &Config{Config: docker.Config{DockerAPIVersion: "1.24", Timeout: 5 * time.Second}, CacheSyncInterval: 5 * time.Second}
	assert.Equal(t, "endpoint must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Config: docker.Config{Endpoint: "someEndpoint", DockerAPIVersion: "1.23"}}
	assert.Equal(t, `"api_version" 1.23 must be at least 1.24`, component.ValidateConfig(cfg).Error())

	cfg = &Config{Config: docker.Config{Endpoint: "someEndpoint", DockerAPIVersion: version}}
	assert.Equal(t, "timeout must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Config: docker.Config{Endpoint: "someEndpoint", DockerAPIVersion: version, Timeout: 5 * time.Minute}}
	assert.Equal(t, "cache_sync_interval must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Config: docker.Config{Endpoint: "someEndpoint", DockerAPIVersion: version, Timeout: 5 * time.Minute}, CacheSyncInterval: 5 * time.Minute}
	assert.NoError(t, component.ValidateConfig(cfg))
}

func loadConf(tb testing.TB, path string, id component.ID) *confmap.Conf {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", path))
	require.NoError(tb, err)
	sub, err := cm.Sub(id.String())
	require.NoError(tb, err)
	return sub
}

func loadConfig(tb testing.TB, id component.ID) *Config {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub := loadConf(tb, "config.yaml", id)
	require.NoError(tb, sub.Unmarshal(cfg))
	return cfg.(*Config)
}

func TestApiVersionCustomError(t *testing.T) {
	sub := loadConf(t, "api_version_float.yaml", component.NewID(metadata.Type))
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := sub.Unmarshal(cfg)
	assert.ErrorContains(t, err,
		`Hint: You may want to wrap the 'api_version' value in quotes (api_version: "1.40")`,
	)

	sub = loadConf(t, "api_version_string.yaml", component.NewID(metadata.Type))
	err = sub.Unmarshal(cfg)
	require.NoError(t, err)
}
