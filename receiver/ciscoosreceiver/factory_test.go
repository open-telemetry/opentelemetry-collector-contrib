// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "ciscoosreceiver", factory.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	require.True(t, ok)

	// Verify defaults from scraperhelper.NewDefaultControllerConfig()
	assert.Equal(t, 60*time.Second, config.CollectionInterval, "Default collection interval should be 60s")
	assert.Equal(t, 0*time.Second, config.Timeout, "Default timeout should be 0s (no timeout)")
	assert.Empty(t, config.Devices, "Default config should have no devices")
	assert.Empty(t, config.Scrapers, "Default config should have no scrapers")
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	// With default (empty) config, should return nop receiver
	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}

func TestFactoryCanBeUsed(t *testing.T) {
	factory := NewFactory()
	err := componenttest.CheckConfigStruct(factory.CreateDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetricsReceiver_WithScrapers(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Add device and scrapers
	cfg.Devices = []DeviceConfig{
		{
			Device: DeviceInfo{Host: HostInfo{IP: "192.168.1.1", Port: 22}},
			Auth:   AuthConfig{Username: "admin", Password: "password"},
		},
	}
	cfg.Scrapers = map[component.Type]component.Config{
		component.MustNewType("system"):     &systemscraper.Config{},
		component.MustNewType("interfaces"): &interfacesscraper.Config{},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}

func TestConvertToSystemScraperDeviceConfigs(t *testing.T) {
	devices := []DeviceConfig{
		{
			Device: DeviceInfo{
				Host: HostInfo{
					Name: "router1",
					IP:   "10.0.0.1",
					Port: 22,
				},
			},
			Auth: AuthConfig{
				Username: "admin",
				Password: "secret",
				KeyFile:  "/path/to/key",
			},
		},
	}

	result := convertToSystemScraperDeviceConfigs(devices)

	require.Len(t, result, 1)
	assert.Equal(t, "router1", result[0].Host.Name)
	assert.Equal(t, "10.0.0.1", result[0].Host.IP)
	assert.Equal(t, 22, result[0].Host.Port)
	assert.Equal(t, "admin", result[0].Auth.Username)
	assert.Equal(t, "secret", result[0].Auth.Password)
	assert.Equal(t, "/path/to/key", result[0].Auth.KeyFile)
}

func TestConvertToInterfaceScraperDeviceConfigs(t *testing.T) {
	devices := []DeviceConfig{
		{
			Device: DeviceInfo{
				Host: HostInfo{
					Name: "switch1",
					IP:   "10.0.0.2",
					Port: 2222,
				},
			},
			Auth: AuthConfig{
				Username: "operator",
				Password: "pass123",
			},
		},
	}

	result := convertToInterfaceScraperDeviceConfigs(devices)

	require.Len(t, result, 1)
	assert.Equal(t, "switch1", result[0].Host.Name)
	assert.Equal(t, "10.0.0.2", result[0].Host.IP)
	assert.Equal(t, 2222, result[0].Host.Port)
	assert.Equal(t, "operator", result[0].Auth.Username)
	assert.Equal(t, "pass123", result[0].Auth.Password)
}

func TestConvertToScraperDeviceConfigs_MultipleDevices(t *testing.T) {
	devices := []DeviceConfig{
		{
			Device: DeviceInfo{Host: HostInfo{IP: "10.0.0.1", Port: 22}},
			Auth:   AuthConfig{Username: "admin1", Password: "pass1"},
		},
		{
			Device: DeviceInfo{Host: HostInfo{IP: "10.0.0.2", Port: 22}},
			Auth:   AuthConfig{Username: "admin2", Password: "pass2"},
		},
		{
			Device: DeviceInfo{Host: HostInfo{IP: "10.0.0.3", Port: 22}},
			Auth:   AuthConfig{Username: "admin3", Password: "pass3"},
		},
	}

	sysResult := convertToSystemScraperDeviceConfigs(devices)
	intResult := convertToInterfaceScraperDeviceConfigs(devices)

	assert.Len(t, sysResult, 3)
	assert.Len(t, intResult, 3)

	// Verify all devices were converted
	assert.Equal(t, "10.0.0.1", sysResult[0].Host.IP)
	assert.Equal(t, "10.0.0.2", sysResult[1].Host.IP)
	assert.Equal(t, "10.0.0.3", sysResult[2].Host.IP)
}

func TestNopMetricsReceiver(t *testing.T) {
	receiver := &nopMetricsReceiver{}

	// Test Start
	err := receiver.Start(t.Context(), nil)
	assert.NoError(t, err)

	// Test Shutdown
	err = receiver.Shutdown(t.Context())
	assert.NoError(t, err)
}
