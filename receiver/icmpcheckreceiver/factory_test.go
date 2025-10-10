// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package icmpcheckreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icmpcheckreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "icmpcheckreceiver", factory.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Empty(t, config.Targets)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Add a device to make config valid
	config := cfg.(*Config)
	config.Targets = []PingTarget{
		{Host: "example.com", PingCount: 5, PingTimeout: 1 * time.Second, PingInterval: 10 * time.Second},
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
