// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	// Test Oracle connection defaults
	config := cfg.(*Config)
	assert.Equal(t, 5, config.MaxOpenConnections, "Expected default MaxOpenConnections to be 5")
	assert.False(t, config.DisableConnectionPool, "Expected default DisableConnectionPool to be false")
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Set required config fields for testing
	config := cfg.(*Config)
	config.DataSource = "oracle://user:password@localhost:1521/XE"

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(context.Background(), set, cfg, consumer)
	assert.NoError(t, err, "failed to create receiver")
	assert.NotNil(t, receiver, "receiver creation returned nil")
}

func TestCreateReceiverWithConnectionPool(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Set required config fields with custom connection pool settings
	config := cfg.(*Config)
	config.DataSource = "oracle://user:password@localhost:1521/XE"
	config.MaxOpenConnections = 10
	config.DisableConnectionPool = false

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(context.Background(), set, cfg, consumer)
	assert.NoError(t, err, "failed to create receiver with connection pool settings")
	assert.NotNil(t, receiver, "receiver creation returned nil")
}

func TestCreateReceiverWithDisabledConnectionPool(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Set required config fields with disabled connection pool
	config := cfg.(*Config)
	config.DataSource = "oracle://user:password@localhost:1521/XE"
	config.MaxOpenConnections = 1
	config.DisableConnectionPool = true

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(context.Background(), set, cfg, consumer)
	assert.NoError(t, err, "failed to create receiver with disabled connection pool")
	assert.NotNil(t, receiver, "receiver creation returned nil")
}

func TestCreateReceiverWithInvalidConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Don't set required config fields - should fail validation
	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(context.Background(), set, cfg, consumer)
	assert.Error(t, err, "expected error for invalid config")
	assert.Nil(t, receiver, "receiver should be nil for invalid config")
}
