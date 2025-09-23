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
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	
	// Set required config fields for testing
	config := cfg.(*Config)
	config.DataSource = "oracle://user:password@localhost:1521/XE"

	set := receivertest.NewNopSettings()
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateMetrics(context.Background(), set, cfg, consumer)
	assert.NoError(t, err, "failed to create receiver")
	assert.NotNil(t, receiver, "receiver creation returned nil")
}
