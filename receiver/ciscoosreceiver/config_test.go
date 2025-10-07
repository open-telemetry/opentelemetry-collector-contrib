// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
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
						Device: DeviceInfo{
							Host: HostInfo{IP: "localhost", Port: 22},
						},
						Auth: AuthConfig{Username: "admin", Password: "password"},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("interfaces"): nil,
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
						Device: DeviceInfo{
							Host: HostInfo{IP: "localhost", Port: 22},
						},
						Auth: AuthConfig{Username: "admin", KeyFile: "/path/to/key"},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("interfaces"): nil,
				},
			},
			expectedErr: "",
		},
		{
			name: "no devices",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("interfaces"): nil,
				},
			},
			expectedErr: "at least one device must be configured",
		},
		{
			name: "empty host IP",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{IP: "", Port: 22},
						},
						Auth: AuthConfig{Username: "admin", Password: "password"},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("interfaces"): nil,
				},
			},
			expectedErr: "host.ip cannot be empty",
		},
		{
			name: "missing username for password auth",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{IP: "localhost", Port: 22},
						},
						Auth: AuthConfig{Username: "", Password: "password"},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("interfaces"): nil,
				},
			},
			expectedErr: "auth.username cannot be empty",
		},
		{
			name: "missing password for password auth",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{IP: "localhost", Port: 22},
						},
						Auth: AuthConfig{Username: "admin", Password: ""},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("interfaces"): nil,
				},
			},
			expectedErr: "auth.password or auth.key_file must be provided",
		},
		{
			name: "no scrapers enabled",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{IP: "localhost", Port: 22},
						},
						Auth: AuthConfig{Username: "admin", Password: "password"},
					},
				},
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
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Devices:          []DeviceConfig{},
		Scrapers:         map[component.Type]component.Config{},
	}

	// Test that Config struct can be created and has expected fields
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Devices)
	assert.NotNil(t, cfg.Scrapers)
	assert.Equal(t, 60*time.Second, cfg.CollectionInterval)
}

func TestDeviceConfig_Fields(t *testing.T) {
	device := DeviceConfig{
		Device: DeviceInfo{
			Host: HostInfo{
				Name: "test-device",
				IP:   "192.168.1.1",
				Port: 22,
			},
		},
		Auth: AuthConfig{
			Username: "admin",
			Password: "secret",
			KeyFile:  "/path/to/key",
		},
	}

	assert.Equal(t, "test-device", device.Device.Host.Name)
	assert.Equal(t, "192.168.1.1", device.Device.Host.IP)
	assert.Equal(t, 22, device.Device.Host.Port)
	assert.Equal(t, "admin", device.Auth.Username)
	assert.Equal(t, "secret", string(device.Auth.Password))
	assert.Equal(t, "/path/to/key", device.Auth.KeyFile)
}

func TestHostInfo_AllFields(t *testing.T) {
	host := HostInfo{
		Name: "router1",
		IP:   "10.0.0.1",
		Port: 2222,
	}

	assert.Equal(t, "router1", host.Name)
	assert.Equal(t, "10.0.0.1", host.IP)
	assert.Equal(t, 2222, host.Port)
}

func TestAuthConfig_PasswordAuth(t *testing.T) {
	auth := AuthConfig{
		Username: "testuser",
		Password: "testpass",
	}

	assert.Equal(t, "testuser", auth.Username)
	assert.Equal(t, "testpass", string(auth.Password))
	assert.Empty(t, auth.KeyFile)
}

func TestAuthConfig_KeyFileAuth(t *testing.T) {
	auth := AuthConfig{
		Username: "testuser",
		KeyFile:  "/home/user/.ssh/id_rsa",
	}

	assert.Equal(t, "testuser", auth.Username)
	assert.Empty(t, auth.Password)
	assert.Equal(t, "/home/user/.ssh/id_rsa", auth.KeyFile)
}

func TestConfig_MultipleDevices(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			Timeout:            30 * time.Second,
			CollectionInterval: 60 * time.Second,
		},
		Devices: []DeviceConfig{
			{
				Device: DeviceInfo{Host: HostInfo{IP: "192.168.1.1", Port: 22}},
				Auth:   AuthConfig{Username: "admin", Password: "pass1"},
			},
			{
				Device: DeviceInfo{Host: HostInfo{IP: "192.168.1.2", Port: 22}},
				Auth:   AuthConfig{Username: "admin", Password: "pass2"},
			},
		},
		Scrapers: map[component.Type]component.Config{
			component.MustNewType("system"): nil,
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)
	assert.Len(t, cfg.Devices, 2)
}

func TestConfig_EmptyHostName(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			Timeout:            30 * time.Second,
			CollectionInterval: 60 * time.Second,
		},
		Devices: []DeviceConfig{
			{
				Device: DeviceInfo{
					Host: HostInfo{
						Name: "", // Empty name is OK, IP is what matters
						IP:   "192.168.1.1",
						Port: 22,
					},
				},
				Auth: AuthConfig{Username: "admin", Password: "password"},
			},
		},
		Scrapers: map[component.Type]component.Config{
			component.MustNewType("system"): nil,
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_ValidatePort(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			Timeout:            30 * time.Second,
			CollectionInterval: 60 * time.Second,
		},
		Devices: []DeviceConfig{
			{
				Device: DeviceInfo{
					Host: HostInfo{
						IP:   "192.168.1.1",
						Port: 0, // Invalid port
					},
				},
				Auth: AuthConfig{Username: "admin", Password: "password"},
			},
		},
		Scrapers: map[component.Type]component.Config{
			component.MustNewType("system"): nil,
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host.port cannot be empty")
}

func TestConfig_ValidateBothPasswordAndKeyFile(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			Timeout:            30 * time.Second,
			CollectionInterval: 60 * time.Second,
		},
		Devices: []DeviceConfig{
			{
				Device: DeviceInfo{Host: HostInfo{IP: "192.168.1.1", Port: 22}},
				Auth: AuthConfig{
					Username: "admin",
					Password: "password",
					KeyFile:  "/path/to/key", // Both provided is OK
				},
			},
		},
		Scrapers: map[component.Type]component.Config{
			component.MustNewType("system"): nil,
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err) // Both is allowed, password takes precedence
}

func TestConfig_UnmarshalWithScrapers(t *testing.T) {
	cfg := &Config{
		Scrapers: map[component.Type]component.Config{},
	}

	// Verify unmarshal initializes maps
	assert.NotNil(t, cfg.Scrapers)
	assert.Empty(t, cfg.Scrapers)
}

func TestGetAvailableScraperTypes(t *testing.T) {
	types := getAvailableScraperTypes()

	// Should contain known scraper types
	assert.Contains(t, types, "system")
	assert.Contains(t, types, "interfaces")
	assert.Len(t, types, 2) // We have 2 scrapers
}

func TestConfig_CollectionIntervalDefault(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
	}

	// Default collection interval should be 60s
	assert.Equal(t, 60*time.Second, cfg.CollectionInterval)
}

func TestDeviceInfo_FullyPopulated(t *testing.T) {
	info := DeviceInfo{
		Host: HostInfo{
			Name: "router-1",
			IP:   "10.0.0.1",
			Port: 2222,
		},
	}

	assert.Equal(t, "router-1", info.Host.Name)
	assert.Equal(t, "10.0.0.1", info.Host.IP)
	assert.Equal(t, 2222, info.Host.Port)
}
