// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiverMetricsFirst(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = "localhost:1" // Endpoint is required, not going to be used here.

	params := receivertest.NewNopSettings(metadata.Type)
	mReceiver, err := factory.CreateMetrics(t.Context(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	_, err = factory.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	assert.ErrorIs(t, err, pipeline.ErrSignalNotSupported)

	lReceiver, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	assert.Same(t, mReceiver, lReceiver)
}

func TestCreateReceiverLogsFirst(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = "localhost:1" // Endpoint is required, not going to be used here.

	lReceiver, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	params := receivertest.NewNopSettings(metadata.Type)
	mReceiver, err := factory.CreateMetrics(t.Context(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	assert.Same(t, mReceiver, lReceiver)
}
