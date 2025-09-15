// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "newrelicsqlserver", factory.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	// Validate it's our config type
	config, ok := cfg.(*Config)
	require.True(t, ok)

	// Check some default values
	assert.Equal(t, "127.0.0.1", config.Hostname)
	assert.Equal(t, "1433", config.Port)
	assert.True(t, config.EnableBufferMetrics)
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// This test creates the receiver - connection errors will happen during Start()
	receiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(factory.Type()),
		cfg,
		consumertest.NewNop(),
	)

	// The factory should successfully create the receiver
	// Connection errors happen when the receiver starts, not during creation
	require.NoError(t, err)
	require.NotNil(t, receiver)
}
