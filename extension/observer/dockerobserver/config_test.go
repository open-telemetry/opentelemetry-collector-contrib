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
				Endpoint:              "unix:///var/run/docker.sock",
				CacheSyncInterval:     5 * time.Minute,
				Timeout:               20 * time.Second,
				ExcludedImages:        []string{"excluded", "image"},
				UseHostnameIfPresent:  true,
				UseHostBindings:       true,
				IgnoreNonHostBindings: true,
				DockerAPIVersion:      version,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "unsupported_api_version"),
			expected: &Config{
				Endpoint:          "unix:///var/run/docker.sock",
				CacheSyncInterval: time.Hour,
				Timeout:           5 * time.Second,
				DockerAPIVersion:  "1.4",
			},
			expectedError: `"api_version" 1.4 must be at least 1.22`,
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
	cfg := &Config{}
	assert.Equal(t, "endpoint must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: "1.21"}
	assert.Equal(t, `"api_version" 1.21 must be at least 1.22`, component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: version}
	assert.Equal(t, "timeout must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: version, Timeout: 5 * time.Minute}
	assert.Equal(t, "cache_sync_interval must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: version, Timeout: 5 * time.Minute, CacheSyncInterval: 5 * time.Minute}
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
