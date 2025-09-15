// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	assert.Equal(t, metadata.Type, factory.Type())
	// Test that factory can create receivers
	assert.NotNil(t, factory)
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	assert.True(t, ok)

	// Test default values
	assert.Equal(t, "localhost", config.Hostname)
	assert.Equal(t, "1433", config.Port)
	assert.Equal(t, "", config.Username)
	assert.Equal(t, "", config.Password)
	assert.Equal(t, true, config.EnableBufferMetrics)
	assert.Equal(t, 5, config.MaxConcurrentWorkers)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 10*time.Second, config.ControllerConfig.CollectionInterval)
}

func TestFactoryFunctions(t *testing.T) {
	// Test that factory functions work correctly
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)

	config := cfg.(*Config)
	assert.Equal(t, "localhost", config.Hostname)
	assert.Equal(t, "1433", config.Port)
}
