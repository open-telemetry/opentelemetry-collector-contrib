// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

func TestEnableCreatedTimestampZeroIngestionGateUsage(t *testing.T) {

	ctx := context.Background()
	mockConsumer := new(consumertest.MetricsSink)
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)

	// Test with feature gate enabled
	err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", true)
	require.NoError(t, err)
	r1, err := newPrometheusReceiver(settings, cfg, mockConsumer)
	require.NoError(t, err)

	assert.True(t, enableCreatedTimestampZeroIngestionGate.IsEnabled(), "Feature gate should be enabled")
	opts := r1.initScrapeOptions()
	assert.True(t, opts.EnableCreatedTimestampZeroIngestion, "EnableCreatedTimestampZeroIngestion should be true when feature gate is enabled")

	// Test with feature gate disabled
	err = featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", false)
	require.NoError(t, err)
	r2, err := newPrometheusReceiver(settings, cfg, mockConsumer)
	require.NoError(t, err)

	assert.False(t, enableCreatedTimestampZeroIngestionGate.IsEnabled(), "Feature gate should be disabled")
	opts = r2.initScrapeOptions()
	assert.False(t, opts.EnableCreatedTimestampZeroIngestion, "EnableCreatedTimestampZeroIngestion should be false when feature gate is disabled")

	// Reset the feature gate and shutdown the created receivers
	t.Cleanup(func() {
		err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", false)
		require.NoError(t, err, "Failed to reset feature gate to default state")

		require.NoError(t, r1.Shutdown(ctx), "Failed to shutdown receiver 1")
		require.NoError(t, r2.Shutdown(ctx), "Failed to shutdown receiver 2")
	})
}
