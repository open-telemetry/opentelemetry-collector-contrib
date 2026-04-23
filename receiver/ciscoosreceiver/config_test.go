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

func newTestDeviceConfig(host string, port int, auth connection.AuthConfig) DeviceConfig {
	return DeviceConfig{
		Name: "test-device",
		Host: host,
		Port: port,
		Auth: auth,
	}
}

func validTestDevice() DeviceConfig {
	return newTestDeviceConfig("192.168.1.1", 22, connection.AuthConfig{
		Username: "admin",
		Password: configopaque.String("password"),
	})
}

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
				Devices: []DeviceConfig{validTestDevice()},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
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
					newTestDeviceConfig("192.168.1.1", 22, connection.AuthConfig{
						Username: "admin",
						KeyFile:  "/path/to/key",
					}),
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
				},
			},
			expectedErr: "",
		},
		{
			name: "empty device host",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					newTestDeviceConfig("", 22, connection.AuthConfig{
						Username: "admin",
						Password: configopaque.String("password"),
					}),
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
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
					newTestDeviceConfig("192.168.1.1", 0, connection.AuthConfig{
						Username: "admin",
						Password: configopaque.String("password"),
					}),
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
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
					newTestDeviceConfig("192.168.1.1", 22, connection.AuthConfig{
						Username: "",
						Password: configopaque.String("password"),
					}),
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
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
					newTestDeviceConfig("192.168.1.1", 22, connection.AuthConfig{
						Username: "admin",
					}),
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
				},
			},
			expectedErr: "devices[0].auth.password or devices[0].auth.key_file must be provided",
		},
		{
			name: "no devices configured",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
				},
			},
			expectedErr: "must specify at least one device",
		},
		{
			name: "no scrapers configured",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices:  []DeviceConfig{validTestDevice()},
				Scrapers: map[component.Type]component.Config{},
			},
			expectedErr: "must specify at least one scraper",
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
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	sub, err := cm.Sub("ciscoos")
	require.NoError(t, err)

	require.NoError(t, sub.Unmarshal(cfg))
	require.Len(t, cfg.Devices, 2)
	assert.Len(t, cfg.Scrapers, 2)
	assert.Contains(t, cfg.Scrapers, component.MustNewType("system"))
	assert.Contains(t, cfg.Scrapers, component.MustNewType("interfaces"))
}

func TestConfigUnmarshalNil(t *testing.T) {
	cfg := &Config{}
	err := cfg.Unmarshal(nil)
	require.NoError(t, err)
}
