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
	assert.Empty(t, config.Device.Device.Host.IP)
	assert.Empty(t, config.Scrapers)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Add a device and scraper to make config valid
	config := cfg.(*Config)
	config.Device = connection.DeviceConfig{
		Device: connection.DeviceInfo{
			Host: connection.HostInfo{
				Name: "test-device",
				IP:   "192.168.1.1",
				Port: 22,
			},
		},
		Auth: connection.AuthConfig{
			Username: "admin",
			Password: configopaque.String("password"),
		},
	}
	// Add system scraper with default config
	systemFactory := systemscraper.NewFactory()
	config.Scrapers = map[component.Type]component.Config{
		component.MustNewType("system"): systemFactory.CreateDefaultConfig(),
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	// For skeleton, we expect a no-op receiver and no error
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

	// Add a device and interfaces scraper
	config := cfg.(*Config)
	config.Device = connection.DeviceConfig{
		Device: connection.DeviceInfo{
			Host: connection.HostInfo{
				Name: "test-device",
				IP:   "192.168.1.1",
				Port: 22,
			},
		},
		Auth: connection.AuthConfig{
			Username: "admin",
			Password: configopaque.String("password"),
		},
	}
	// Add interfaces scraper with default config
	interfacesFactory := interfacesscraper.NewFactory()
	config.Scrapers = map[component.Type]component.Config{
		component.MustNewType("interfaces"): interfacesFactory.CreateDefaultConfig(),
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

	// Add a device and both scrapers
	config := cfg.(*Config)
	config.Device = connection.DeviceConfig{
		Device: connection.DeviceInfo{
			Host: connection.HostInfo{
				Name: "test-device",
				IP:   "192.168.1.1",
				Port: 22,
			},
		},
		Auth: connection.AuthConfig{
			Username: "admin",
			Password: configopaque.String("password"),
		},
	}
	// Add both system and interfaces scrapers
	systemFactory := systemscraper.NewFactory()
	interfacesFactory := interfacesscraper.NewFactory()
	config.Scrapers = map[component.Type]component.Config{
		component.MustNewType("system"):     systemFactory.CreateDefaultConfig(),
		component.MustNewType("interfaces"): interfacesFactory.CreateDefaultConfig(),
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}

func TestScraperFactoriesRegistered(t *testing.T) {
	// Verify both scrapers are registered
	assert.Contains(t, scraperFactories, component.MustNewType("system"))
	assert.Contains(t, scraperFactories, component.MustNewType("interfaces"))
	assert.Len(t, scraperFactories, 2)
}
