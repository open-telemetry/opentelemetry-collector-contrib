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
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "ciscoos", factory.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, 1*time.Minute, config.CollectionInterval)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Empty(t, config.Devices)
	assert.Empty(t, config.Scrapers)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Add a device with system scraper to make config valid
	config := cfg.(*Config)
	systemFactory := systemscraper.NewFactory()
	config.Devices = []DeviceConfig{
		{
			Name: "test-device",
			Host: "192.168.1.1",
			Port: 22,
			Auth: connection.AuthConfig{
				Username: "admin",
				Password: configopaque.String("password"),
			},
			Scrapers: map[component.Type]component.Config{
				component.MustNewType("system"): systemFactory.CreateDefaultConfig(),
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}

func TestFactoryCanBeUsed(t *testing.T) {
	factory := NewFactory()
	err := componenttest.CheckConfigStruct(factory.CreateDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetricsReceiverWithInterfacesScraper(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Add a device with interfaces scraper
	config := cfg.(*Config)
	interfacesFactory := interfacesscraper.NewFactory()
	config.Devices = []DeviceConfig{
		{
			Name: "test-device",
			Host: "192.168.1.1",
			Port: 22,
			Auth: connection.AuthConfig{
				Username: "admin",
				Password: configopaque.String("password"),
			},
			Scrapers: map[component.Type]component.Config{
				component.MustNewType("interfaces"): interfacesFactory.CreateDefaultConfig(),
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}

func TestCreateMetricsReceiverWithBothScrapers(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Add a device with both scrapers
	config := cfg.(*Config)
	systemFactory := systemscraper.NewFactory()
	interfacesFactory := interfacesscraper.NewFactory()
	config.Devices = []DeviceConfig{
		{
			Name: "test-device",
			Host: "192.168.1.1",
			Port: 22,
			Auth: connection.AuthConfig{
				Username: "admin",
				Password: configopaque.String("password"),
			},
			Scrapers: map[component.Type]component.Config{
				component.MustNewType("system"):     systemFactory.CreateDefaultConfig(),
				component.MustNewType("interfaces"): interfacesFactory.CreateDefaultConfig(),
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}

func TestCreateMetricsReceiverWithMultipleDevices(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Add multiple devices
	config := cfg.(*Config)
	systemFactory := systemscraper.NewFactory()
	interfacesFactory := interfacesscraper.NewFactory()
	config.Devices = []DeviceConfig{
		{
			Name: "device-1",
			Host: "192.168.1.1",
			Port: 22,
			Auth: connection.AuthConfig{
				Username: "admin",
				Password: configopaque.String("password"),
			},
			Scrapers: map[component.Type]component.Config{
				component.MustNewType("system"): systemFactory.CreateDefaultConfig(),
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
				component.MustNewType("interfaces"): interfacesFactory.CreateDefaultConfig(),
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)

	// Should return a multiMetricsReceiver for multiple devices
	_, isMulti := receiver.(*multiMetricsReceiver)
	assert.True(t, isMulti, "expected multiMetricsReceiver for multiple devices")
}

func TestScraperFactoriesRegistered(t *testing.T) {
	// Verify both scrapers are registered
	assert.Contains(t, scraperFactories, component.MustNewType("system"))
	assert.Contains(t, scraperFactories, component.MustNewType("interfaces"))
	assert.Len(t, scraperFactories, 2)
}

func TestCreateMetricsReceiverNoDevices(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// No devices configured - should return nopMetricsReceiver
	config := cfg.(*Config)
	config.Devices = []DeviceConfig{}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)

	// Should return a nopMetricsReceiver
	_, isNop := receiver.(*nopMetricsReceiver)
	assert.True(t, isNop, "expected nopMetricsReceiver for no devices")
}

func TestCreateMetricsReceiverDeviceWithMissingHost(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Device with empty host should be skipped
	config := cfg.(*Config)
	systemFactory := systemscraper.NewFactory()
	config.Devices = []DeviceConfig{
		{
			Name: "device-no-host",
			Host: "", // Missing host
			Port: 22,
			Auth: connection.AuthConfig{
				Username: "admin",
				Password: configopaque.String("password"),
			},
			Scrapers: map[component.Type]component.Config{
				component.MustNewType("system"): systemFactory.CreateDefaultConfig(),
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)

	// Should return nopMetricsReceiver since the only device was skipped
	_, isNop := receiver.(*nopMetricsReceiver)
	assert.True(t, isNop, "expected nopMetricsReceiver when device is skipped")
}

func TestCreateMetricsReceiverDeviceWithEmptyScrapers(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Device with empty scrapers should be skipped
	config := cfg.(*Config)
	config.Devices = []DeviceConfig{
		{
			Name: "device-no-scrapers",
			Host: "192.168.1.1",
			Port: 22,
			Auth: connection.AuthConfig{
				Username: "admin",
				Password: configopaque.String("password"),
			},
			Scrapers: map[component.Type]component.Config{}, // Empty scrapers
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)

	// Should return nopMetricsReceiver since the only device was skipped
	_, isNop := receiver.(*nopMetricsReceiver)
	assert.True(t, isNop, "expected nopMetricsReceiver when device has empty scrapers")
}

func TestCreateMetricsReceiverSingleDevice(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Single device should return the controller directly, not multiMetricsReceiver
	config := cfg.(*Config)
	systemFactory := systemscraper.NewFactory()
	config.Devices = []DeviceConfig{
		{
			Name: "single-device",
			Host: "192.168.1.1",
			Port: 22,
			Auth: connection.AuthConfig{
				Username: "admin",
				Password: configopaque.String("password"),
			},
			Scrapers: map[component.Type]component.Config{
				component.MustNewType("system"): systemFactory.CreateDefaultConfig(),
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)

	// Should NOT be multiMetricsReceiver for single device
	_, isMulti := receiver.(*multiMetricsReceiver)
	assert.False(t, isMulti, "expected single receiver, not multiMetricsReceiver for one device")

	// Should NOT be nopMetricsReceiver
	_, isNop := receiver.(*nopMetricsReceiver)
	assert.False(t, isNop, "expected real receiver, not nopMetricsReceiver")
}

func TestNopMetricsReceiverLifecycle(t *testing.T) {
	nop := &nopMetricsReceiver{}

	// Test Start
	err := nop.Start(t.Context(), nil)
	assert.NoError(t, err)

	// Test Shutdown
	err = nop.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestMultiMetricsReceiverLifecycle(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	config := cfg.(*Config)
	systemFactory := systemscraper.NewFactory()
	interfacesFactory := interfacesscraper.NewFactory()
	config.Devices = []DeviceConfig{
		{
			Name: "device-1",
			Host: "192.168.1.1",
			Port: 22,
			Auth: connection.AuthConfig{
				Username: "admin",
				Password: configopaque.String("password"),
			},
			Scrapers: map[component.Type]component.Config{
				component.MustNewType("system"): systemFactory.CreateDefaultConfig(),
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
				component.MustNewType("interfaces"): interfacesFactory.CreateDefaultConfig(),
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	require.NoError(t, err)

	multi, isMulti := receiver.(*multiMetricsReceiver)
	require.True(t, isMulti)

	// Test Start
	err = multi.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)

	// Test Shutdown
	err = multi.Shutdown(t.Context())
	assert.NoError(t, err)
}
