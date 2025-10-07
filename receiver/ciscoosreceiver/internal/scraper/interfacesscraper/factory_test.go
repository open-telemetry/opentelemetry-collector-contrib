// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
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
	assert.True(t, config.Metrics.CiscoInterfaceReceiveBytes.Enabled)
	assert.True(t, config.Metrics.CiscoInterfaceTransmitBytes.Enabled)
	assert.True(t, config.Metrics.CiscoInterfaceUp.Enabled)
}

func TestFactory_ConfigWithDevices(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Add device configuration
	cfg.Devices = []DeviceConfig{
		{
			Host: HostInfo{
				IP:   "192.168.1.1",
				Port: 22,
			},
			Auth: AuthConfig{
				Username: "admin",
				Password: "password",
			},
		},
	}

	// Verify config structure is correct
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.Devices, 1)
	assert.Equal(t, "192.168.1.1", cfg.Devices[0].Host.IP)
	assert.Equal(t, 22, cfg.Devices[0].Host.Port)
}

func TestConfig_AllMetricsEnabled(t *testing.T) {
	config := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	// Verify all interface metrics are enabled by default
	metrics := config.Metrics
	assert.True(t, metrics.CiscoInterfaceReceiveBytes.Enabled)
	assert.True(t, metrics.CiscoInterfaceTransmitBytes.Enabled)
	assert.True(t, metrics.CiscoInterfaceUp.Enabled)
	assert.True(t, metrics.CiscoInterfaceReceiveErrors.Enabled)
	assert.True(t, metrics.CiscoInterfaceTransmitErrors.Enabled)
	assert.True(t, metrics.CiscoInterfaceReceiveDrops.Enabled)
	assert.True(t, metrics.CiscoInterfaceTransmitDrops.Enabled)
	assert.True(t, metrics.CiscoInterfaceReceiveMulticast.Enabled)
	assert.True(t, metrics.CiscoInterfaceReceiveBroadcast.Enabled)
	assert.True(t, metrics.CiscoInterfaceErrorStatus.Enabled)
}

func TestConfig_DisableMetrics(t *testing.T) {
	config := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	// Test disabling specific metrics
	config.Metrics.CiscoInterfaceReceiveBytes.Enabled = false
	config.Metrics.CiscoInterfaceTransmitBytes.Enabled = false

	assert.False(t, config.Metrics.CiscoInterfaceReceiveBytes.Enabled)
	assert.False(t, config.Metrics.CiscoInterfaceTransmitBytes.Enabled)
	// Others should still be enabled
	assert.True(t, config.Metrics.CiscoInterfaceUp.Enabled)
}

func TestDeviceConfig_Structure(t *testing.T) {
	device := DeviceConfig{
		Host: HostInfo{
			Name: "switch1",
			IP:   "10.0.0.1",
			Port: 22,
		},
		Auth: AuthConfig{
			Username: "netadmin",
			Password: "netpass",
			KeyFile:  "/etc/ssh/key",
		},
	}

	assert.Equal(t, "switch1", device.Host.Name)
	assert.Equal(t, "10.0.0.1", device.Host.IP)
	assert.Equal(t, 22, device.Host.Port)
	assert.Equal(t, "netadmin", device.Auth.Username)
	assert.Equal(t, "netpass", device.Auth.Password)
	assert.Equal(t, "/etc/ssh/key", device.Auth.KeyFile)
}

func TestConfig_MultipleDevices(t *testing.T) {
	config := &Config{
		Devices: []DeviceConfig{
			{
				Host: HostInfo{Name: "switch1", IP: "10.0.0.1", Port: 22},
				Auth: AuthConfig{Username: "admin1", Password: "pass1"},
			},
			{
				Host: HostInfo{Name: "switch2", IP: "10.0.0.2", Port: 22},
				Auth: AuthConfig{Username: "admin2", Password: "pass2"},
			},
			{
				Host: HostInfo{Name: "router1", IP: "10.0.0.3", Port: 2222},
				Auth: AuthConfig{Username: "admin3", KeyFile: "/path/key"},
			},
		},
	}

	assert.Len(t, config.Devices, 3)
	assert.Equal(t, "switch1", config.Devices[0].Host.Name)
	assert.Equal(t, "switch2", config.Devices[1].Host.Name)
	assert.Equal(t, "router1", config.Devices[2].Host.Name)
	assert.Equal(t, 2222, config.Devices[2].Host.Port)
}

func TestHostInfo_CustomPort(t *testing.T) {
	host := HostInfo{
		Name: "device1",
		IP:   "192.168.100.1",
		Port: 2222,
	}

	assert.Equal(t, "device1", host.Name)
	assert.Equal(t, "192.168.100.1", host.IP)
	assert.Equal(t, 2222, host.Port)
}

func TestAuthConfig_PasswordAuth(t *testing.T) {
	auth := AuthConfig{
		Username: "operator",
		Password: "secret123",
	}

	assert.Equal(t, "operator", auth.Username)
	assert.Equal(t, "secret123", auth.Password)
	assert.Empty(t, auth.KeyFile)
}

func TestAuthConfig_KeyFileAuth(t *testing.T) {
	auth := AuthConfig{
		Username: "sshuser",
		KeyFile:  "/home/user/.ssh/cisco_key",
	}

	assert.Equal(t, "sshuser", auth.Username)
	assert.Empty(t, auth.Password)
	assert.Equal(t, "/home/user/.ssh/cisco_key", auth.KeyFile)
}

func TestAuthConfig_BothAuthMethods(t *testing.T) {
	auth := AuthConfig{
		Username: "admin",
		Password: "pass",
		KeyFile:  "/path/key",
	}

	// Both can be configured, password takes precedence
	assert.Equal(t, "admin", auth.Username)
	assert.Equal(t, "pass", auth.Password)
	assert.Equal(t, "/path/key", auth.KeyFile)
}

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	scraperType := factory.Type()

	assert.Equal(t, "interfaces", scraperType.String())
}

func TestConfig_EmptyDevices(t *testing.T) {
	config := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []DeviceConfig{},
	}

	assert.NotNil(t, config)
	assert.Empty(t, config.Devices)
	assert.NotNil(t, config.MetricsBuilderConfig)
}

func TestCreateDefaultConfig_AllInterfaceMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Verify all 10 interface metrics are enabled
	metrics := cfg.Metrics
	assert.True(t, metrics.CiscoInterfaceReceiveBytes.Enabled)
	assert.True(t, metrics.CiscoInterfaceTransmitBytes.Enabled)
	assert.True(t, metrics.CiscoInterfaceUp.Enabled)
	assert.True(t, metrics.CiscoInterfaceReceiveErrors.Enabled)
	assert.True(t, metrics.CiscoInterfaceTransmitErrors.Enabled)
	assert.True(t, metrics.CiscoInterfaceReceiveDrops.Enabled)
	assert.True(t, metrics.CiscoInterfaceTransmitDrops.Enabled)
	assert.True(t, metrics.CiscoInterfaceReceiveMulticast.Enabled)
	assert.True(t, metrics.CiscoInterfaceReceiveBroadcast.Enabled)
	assert.True(t, metrics.CiscoInterfaceErrorStatus.Enabled)
}

func TestHostInfo_EmptyName(t *testing.T) {
	host := HostInfo{
		Name: "", // Name is optional
		IP:   "10.1.1.1",
		Port: 22,
	}

	assert.Empty(t, host.Name)
	assert.Equal(t, "10.1.1.1", host.IP)
	assert.Equal(t, 22, host.Port)
}
