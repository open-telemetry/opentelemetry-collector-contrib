// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "interfaces", factory.Type().String())
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
	assert.True(t, config.Metrics.SystemNetworkIo.Enabled)
	assert.True(t, config.Metrics.SystemNetworkErrors.Enabled)
	assert.True(t, config.Metrics.SystemNetworkInterfaceStatus.Enabled)
}

func TestFactory_ConfigWithDevice(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Add device configuration
	cfg.Device = connection.DeviceConfig{
		Device: connection.DeviceInfo{
			Host: connection.HostInfo{
				IP:   "192.168.1.1",
				Port: 22,
			},
		},
		Auth: connection.AuthConfig{
			Username: "admin",
			Password: "password",
		},
	}

	// Verify config structure is correct
	assert.NotNil(t, cfg)
	assert.Equal(t, "192.168.1.1", cfg.Device.Device.Host.IP)
	assert.Equal(t, 22, cfg.Device.Device.Host.Port)
}

func TestConfig_AllMetricsEnabled(t *testing.T) {
	config := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	// Verify all interface metrics are enabled by default
	metrics := config.Metrics
	assert.True(t, metrics.SystemNetworkIo.Enabled)
	assert.True(t, metrics.SystemNetworkErrors.Enabled)
	assert.True(t, metrics.SystemNetworkPacketDropped.Enabled)
	assert.True(t, metrics.SystemNetworkPacketCount.Enabled)
	assert.True(t, metrics.SystemNetworkInterfaceStatus.Enabled)
}

func TestConfig_DisableMetrics(t *testing.T) {
	config := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	// Test disabling specific metrics
	config.Metrics.SystemNetworkIo.Enabled = false
	config.Metrics.SystemNetworkErrors.Enabled = false

	assert.False(t, config.Metrics.SystemNetworkIo.Enabled)
	assert.False(t, config.Metrics.SystemNetworkErrors.Enabled)
	// Others should still be enabled
	assert.True(t, config.Metrics.SystemNetworkInterfaceStatus.Enabled)
}

func TestDeviceConfig_Structure(t *testing.T) {
	device := connection.DeviceConfig{
		Device: connection.DeviceInfo{
			Host: connection.HostInfo{
				Name: "switch1",
				IP:   "10.0.0.1",
				Port: 22,
			},
		},
		Auth: connection.AuthConfig{
			Username: "netadmin",
			Password: "netpass",
			KeyFile:  "/etc/ssh/key",
		},
	}

	assert.Equal(t, "switch1", device.Device.Host.Name)
	assert.Equal(t, "10.0.0.1", device.Device.Host.IP)
	assert.Equal(t, 22, device.Device.Host.Port)
	assert.Equal(t, "netadmin", device.Auth.Username)
	assert.Equal(t, "netpass", string(device.Auth.Password))
	assert.Equal(t, "/etc/ssh/key", device.Auth.KeyFile)
}

func TestConfig_SingleDevice(t *testing.T) {
	config := &Config{
		Device: connection.DeviceConfig{
			Device: connection.DeviceInfo{
				Host: connection.HostInfo{Name: "switch1", IP: "10.0.0.1", Port: 22},
			},
			Auth: connection.AuthConfig{Username: "admin1", Password: "pass1"},
		},
	}

	assert.Equal(t, "switch1", config.Device.Device.Host.Name)
	assert.Equal(t, "10.0.0.1", config.Device.Device.Host.IP)
	assert.Equal(t, 22, config.Device.Device.Host.Port)
	assert.Equal(t, "admin1", config.Device.Auth.Username)
}

func TestHostInfo_CustomPort(t *testing.T) {
	host := connection.HostInfo{
		Name: "device1",
		IP:   "192.168.100.1",
		Port: 2222,
	}

	assert.Equal(t, "device1", host.Name)
	assert.Equal(t, "192.168.100.1", host.IP)
	assert.Equal(t, 2222, host.Port)
}

func TestAuthConfig_PasswordAuth(t *testing.T) {
	auth := connection.AuthConfig{
		Username: "operator",
		Password: "secret123",
	}

	assert.Equal(t, "operator", auth.Username)
	assert.Equal(t, "secret123", string(auth.Password))
	assert.Empty(t, auth.KeyFile)
}

func TestAuthConfig_KeyFileAuth(t *testing.T) {
	auth := connection.AuthConfig{
		Username: "sshuser",
		KeyFile:  "/home/user/.ssh/cisco_key",
	}

	assert.Equal(t, "sshuser", auth.Username)
	assert.Empty(t, auth.Password)
	assert.Equal(t, "/home/user/.ssh/cisco_key", auth.KeyFile)
}

func TestAuthConfig_BothAuthMethods(t *testing.T) {
	auth := connection.AuthConfig{
		Username: "admin",
		Password: "pass",
		KeyFile:  "/path/key",
	}

	// Both can be configured, password takes precedence
	assert.Equal(t, "admin", auth.Username)
	assert.Equal(t, "pass", string(auth.Password))
	assert.Equal(t, "/path/key", auth.KeyFile)
}

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	scraperType := factory.Type()

	assert.Equal(t, "interfaces", scraperType.String())
}

func TestConfig_EmptyDevice(t *testing.T) {
	config := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Device:               connection.DeviceConfig{},
	}

	assert.NotNil(t, config)
	assert.Empty(t, config.Device.Device.Host.IP)
	assert.NotNil(t, config.MetricsBuilderConfig)
}

func TestCreateDefaultConfig_AllInterfaceMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Verify all 5 interface metrics are enabled
	metrics := cfg.Metrics
	assert.True(t, metrics.SystemNetworkIo.Enabled)
	assert.True(t, metrics.SystemNetworkErrors.Enabled)
	assert.True(t, metrics.SystemNetworkPacketDropped.Enabled)
	assert.True(t, metrics.SystemNetworkPacketCount.Enabled)
	assert.True(t, metrics.SystemNetworkInterfaceStatus.Enabled)
}

func TestHostInfo_EmptyName(t *testing.T) {
	host := connection.HostInfo{
		Name: "", // Name is optional
		IP:   "10.1.1.1",
		Port: 22,
	}

	assert.Empty(t, host.Name)
	assert.Equal(t, "10.1.1.1", host.IP)
	assert.Equal(t, 22, host.Port)
}
