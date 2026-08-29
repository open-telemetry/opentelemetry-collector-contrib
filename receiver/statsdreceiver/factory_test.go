// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateDefaultConfig_MonotonicCounterDefaultFeatureGate(t *testing.T) {
	gate := metadata.ReceiverStatsdMonotonicCounterDefaultFeatureGate
	originalValue := gate.IsEnabled()
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), originalValue))
	})

	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), false))
	cfg := createDefaultConfig().(*Config)
	assert.False(t, cfg.IsMonotonicCounter)

	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	cfg = createDefaultConfig().(*Config)
	assert.True(t, cfg.IsMonotonicCounter)
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = "localhost:0" // Endpoint is required, not going to be used here.

	params := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := createMetricsReceiver(t.Context(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "receiver creation failed")
}
