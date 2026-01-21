// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "valid config with password auth",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "test-device",
						Host: "192.168.1.1",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("system"): nil,
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "valid config with key file auth",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "test-device",
						Host: "192.168.1.1",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "admin",
							KeyFile:  "/path/to/key",
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("system"): nil,
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "valid config with fallback auth (key + password)",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "test-device",
						Host: "192.168.1.1",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "admin",
							KeyFile:  "/path/to/key",
							Password: configopaque.String("backup-password"),
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("system"): nil,
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "valid config with multiple devices",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "device-1",
						Host: "192.168.1.1",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("system"): nil,
						},
					},
					{
						Name: "device-2",
						Host: "192.168.1.2",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("interfaces"): nil,
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "no devices configured",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{},
			},
			expectedErr: "must specify at least one device",
		},
		{
			name: "empty device host",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "test-device",
						Host: "",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("system"): nil,
						},
					},
				},
			},
			expectedErr: "devices[0].host cannot be empty",
		},
		{
			name: "missing port",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "test-device",
						Host: "192.168.1.1",
						Port: 0,
						Auth: connection.AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("system"): nil,
						},
					},
				},
			},
			expectedErr: "devices[0].port cannot be empty",
		},
		{
			name: "missing username",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "test-device",
						Host: "192.168.1.1",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "",
							Password: configopaque.String("password"),
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("system"): nil,
						},
					},
				},
			},
			expectedErr: "devices[0].auth.username cannot be empty",
		},
		{
			name: "missing password and key file",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "test-device",
						Host: "192.168.1.1",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "admin",
						},
						Scrapers: map[component.Type]component.Config{
							component.MustNewType("system"): nil,
						},
					},
				},
			},
			expectedErr: "devices[0].auth.password or devices[0].auth.key_file must be provided",
		},
		{
			name: "no scrapers configured for device",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Name: "test-device",
						Host: "192.168.1.1",
						Port: 22,
						Auth: connection.AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
						Scrapers: map[component.Type]component.Config{},
					},
				},
			},
			expectedErr: "devices[0] must have at least one scraper",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestConfigUnmarshal(t *testing.T) {
	tests := []struct {
		name           string
		configFile     string
		expectedErr    string
		validateConfig func(t *testing.T, cfg *Config)
	}{
		{
			name:       "global_scrapers_only",
			configFile: "global_scrapers_only.yaml",
			validateConfig: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Devices, 2)
				// Both devices should inherit global scrapers
				assert.Len(t, cfg.Devices[0].Scrapers, 2)
				assert.Len(t, cfg.Devices[1].Scrapers, 2)
				assert.Contains(t, cfg.Devices[0].Scrapers, component.MustNewType("system"))
				assert.Contains(t, cfg.Devices[0].Scrapers, component.MustNewType("interfaces"))
			},
		},
		{
			name:       "per_device_override",
			configFile: "per_device_override.yaml",
			validateConfig: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Devices, 2)
				// Device 1 uses global scrapers (2)
				assert.Len(t, cfg.Devices[0].Scrapers, 2)
				// Device 2 overrides with only system scraper (1)
				assert.Len(t, cfg.Devices[1].Scrapers, 1)
				assert.Contains(t, cfg.Devices[1].Scrapers, component.MustNewType("system"))
			},
		},
		{
			name:       "empty_scrapers_section",
			configFile: "empty_scrapers_section.yaml",
			validateConfig: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Devices, 1)
				// Empty scrapers section means global scrapers are used
				assert.Len(t, cfg.Devices[0].Scrapers, 1)
			},
		},
		{
			name:        "invalid_scraper_key",
			configFile:  "invalid_scraper_key.yaml",
			expectedErr: "invalid scraper key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configFile))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)

			sub, err := cm.Sub("ciscoos")
			require.NoError(t, err)

			err = sub.Unmarshal(cfg)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				return
			}

			require.NoError(t, err)
			if tt.validateConfig != nil {
				tt.validateConfig(t, cfg)
			}
		})
	}
}

func TestConfigUnmarshalNil(t *testing.T) {
	cfg := &Config{}
	err := cfg.Unmarshal(nil)
	require.NoError(t, err)
}

func TestCloneScraperConfigs(t *testing.T) {
	// Test nil input
	result := cloneScraperConfigs(nil)
	assert.Nil(t, result)

	// Test empty map
	empty := make(map[component.Type]component.Config)
	result = cloneScraperConfigs(empty)
	assert.NotNil(t, result)
	assert.Empty(t, result)

	// Test with values
	src := map[component.Type]component.Config{
		component.MustNewType("system"): nil,
	}
	result = cloneScraperConfigs(src)
	assert.Len(t, result, 1)
	assert.Contains(t, result, component.MustNewType("system"))
}

func TestMergeScraperConfigs(t *testing.T) {
	global := map[component.Type]component.Config{
		component.MustNewType("system"):     nil,
		component.MustNewType("interfaces"): nil,
	}

	// Test empty device scrapers - should return clone of global
	result := mergeScraperConfigs(global, nil)
	assert.Len(t, result, 2)

	// Test device scrapers override
	device := map[component.Type]component.Config{
		component.MustNewType("system"): nil,
	}
	result = mergeScraperConfigs(global, device)
	assert.Len(t, result, 1)
	assert.Contains(t, result, component.MustNewType("system"))
}

func TestGetAvailableScraperTypes(t *testing.T) {
	types := getAvailableScraperTypes()
	assert.Contains(t, types, "system")
	assert.Contains(t, types, "interfaces")
	assert.Len(t, types, 2)
}
