// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
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

	// Cast to actual config type and check defaults
	config := cfg.(*Config)
	assert.Empty(t, config.Devices)
}

func TestFactoryCanBeUsed(t *testing.T) {
	factory := NewFactory()
	err := componenttest.CheckConfigStruct(factory.CreateDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)

	// Add basic device configuration
	cfg.Devices = []DeviceConfig{
		{
			Device: DeviceInfo{Host: HostInfo{IP: "192.168.1.1", Port: 22}},
			Auth:   AuthConfig{Username: "admin", Password: "password"},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}
