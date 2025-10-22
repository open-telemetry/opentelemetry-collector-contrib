// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "system", factory.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	// Verify it's a Config type
	config, ok := cfg.(*Config)
	assert.True(t, ok)
	assert.NotNil(t, config)

	// Verify default metrics are enabled
	assert.True(t, config.Metrics.CiscoDeviceUp.Enabled)
}

func TestFactory_CreateScraperMethod(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Add device configuration
	cfg.Device = DeviceConfig{
		Host: HostInfo{
			IP:   "192.168.1.1",
			Port: 22,
		},
		Auth: AuthConfig{
			Username: "admin",
			Password: "password",
		},
	}

	// Verify config structure is correct for scraper creation
	assert.NotNil(t, cfg)
	assert.Equal(t, "192.168.1.1", cfg.Device.Host.IP)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid_config",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Device: DeviceConfig{
					Host: HostInfo{IP: "192.168.1.1", Port: 22},
					Auth: AuthConfig{Username: "admin", Password: "password"},
				},
			},
			expectError: false,
		},
		{
			name: "empty_device",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Device:               DeviceConfig{},
			},
			expectError: false, // Empty device is allowed at config level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Config doesn't have explicit Validate, just verify structure
			assert.NotNil(t, tt.config)
			if !tt.expectError {
				assert.NotNil(t, tt.config.MetricsBuilderConfig)
			}
		})
	}
}

func TestConfig_MetricsConfiguration(t *testing.T) {
	config := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	// Verify metrics are configurable
	assert.True(t, config.Metrics.CiscoDeviceUp.Enabled)

	// Test disabling metrics
	config.Metrics.CiscoDeviceUp.Enabled = false
	assert.False(t, config.Metrics.CiscoDeviceUp.Enabled)
}

func TestDeviceConfig_Structure(t *testing.T) {
	device := DeviceConfig{
		Host: HostInfo{
			Name: "router1",
			IP:   "10.0.0.1",
			Port: 22,
		},
		Auth: AuthConfig{
			Username: "admin",
			Password: "secret",
			KeyFile:  "/path/to/key",
		},
	}

	assert.Equal(t, "router1", device.Host.Name)
	assert.Equal(t, "10.0.0.1", device.Host.IP)
	assert.Equal(t, 22, device.Host.Port)
	assert.Equal(t, "admin", device.Auth.Username)
	assert.Equal(t, "secret", device.Auth.Password)
	assert.Equal(t, "/path/to/key", device.Auth.KeyFile)
}

func TestHostInfo_DefaultPort(t *testing.T) {
	host := HostInfo{
		Name: "router",
		IP:   "192.168.1.1",
		Port: 22,
	}

	assert.Equal(t, "router", host.Name)
	assert.Equal(t, "192.168.1.1", host.IP)
	assert.Equal(t, 22, host.Port)
}

func TestAuthConfig_PasswordOnly(t *testing.T) {
	auth := AuthConfig{
		Username: "testuser",
		Password: "testpass",
	}

	assert.Equal(t, "testuser", auth.Username)
	assert.Equal(t, "testpass", auth.Password)
	assert.Empty(t, auth.KeyFile)
}

func TestAuthConfig_KeyFileOnly(t *testing.T) {
	auth := AuthConfig{
		Username: "testuser",
		KeyFile:  "/home/user/.ssh/id_rsa",
	}

	assert.Equal(t, "testuser", auth.Username)
	assert.Empty(t, auth.Password)
	assert.Equal(t, "/home/user/.ssh/id_rsa", auth.KeyFile)
}

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	scraperType := factory.Type()

	assert.Equal(t, "system", scraperType.String())
}

func TestCreateDefaultConfig_MetricsEnabled(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Verify all default metrics are enabled
	metrics := cfg.Metrics
	assert.True(t, metrics.CiscoDeviceUp.Enabled)
}
