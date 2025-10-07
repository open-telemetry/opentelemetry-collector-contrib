// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			},
			expectedErr: "auth.password or auth.key_file must be provided",
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
	}

	// Test that Config struct can be created and has expected fields
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Devices)
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
	}

	err := cfg.Validate()
	assert.NoError(t, err) // Both is allowed, password takes precedence
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
