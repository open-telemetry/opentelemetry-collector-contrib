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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"
)

func newTestDevice(name, host string) DeviceConfig {
	return DeviceConfig{
		Name: name,
		Host: host,
		Port: 22,
		Auth: connection.AuthConfig{
			Username: "admin",
			Password: configopaque.String("password"),
		},
	}
}

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
	config := factory.CreateDefaultConfig().(*Config)
	config.Devices = []DeviceConfig{newTestDevice("test-device", "192.168.1.1")}
	config.Scrapers = map[component.Type]component.Config{
		component.MustNewType("system"): systemscraper.NewFactory().CreateDefaultConfig(),
	}

	receiver, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), config, consumertest.NewNop())
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}

func TestFactoryCanBeUsed(t *testing.T) {
	factory := NewFactory()
	err := componenttest.CheckConfigStruct(factory.CreateDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetricsReceiverWithMultipleDevices(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Devices = []DeviceConfig{
		newTestDevice("device-1", "192.168.1.1"),
		newTestDevice("device-2", "192.168.1.2"),
	}
	config.Scrapers = map[component.Type]component.Config{
		component.MustNewType("system"): systemscraper.NewFactory().CreateDefaultConfig(),
	}

	receiver, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), config, consumertest.NewNop())
	assert.NotNil(t, receiver)
	assert.NoError(t, err)

	_, isMulti := receiver.(*multiMetricsReceiver)
	assert.True(t, isMulti, "expected multiMetricsReceiver for multiple devices")
}

func TestScraperFactoriesRegistered(t *testing.T) {
	assert.Contains(t, scraperFactories, component.MustNewType("system"))
	assert.Contains(t, scraperFactories, component.MustNewType("interfaces"))
	assert.Len(t, scraperFactories, 2)
}
