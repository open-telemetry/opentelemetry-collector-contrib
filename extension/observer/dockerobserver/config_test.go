// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dockerobserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       config.ComponentID
		expected config.Extension
	}{
		{
			id:       config.NewComponentID(typeStr),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: config.NewComponentIDWithName(typeStr, "all_settings"),
			expected: &Config{
				ExtensionSettings:     config.NewExtensionSettings(config.NewComponentID(typeStr)),
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
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := &Config{}
	assert.Equal(t, "endpoint must be specified", cfg.Validate().Error())

	cfg = &Config{Endpoint: "someEndpoint"}
	assert.Equal(t, "api_version must be at least 1.22", cfg.Validate().Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: 1.22}
	assert.Equal(t, "timeout must be specified", cfg.Validate().Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: 1.22, Timeout: 5 * time.Minute}
	assert.Equal(t, "cache_sync_interval must be specified", cfg.Validate().Error())

	cfg = &Config{Endpoint: "someEndpoint", DockerAPIVersion: 1.22, Timeout: 5 * time.Minute, CacheSyncInterval: 5 * time.Minute}
	assert.Nil(t, cfg.Validate())
}

func loadConfig(t testing.TB, id config.ComponentID) *Config {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	require.NoError(t, config.UnmarshalExtension(sub, cfg))

	return cfg.(*Config)
}
