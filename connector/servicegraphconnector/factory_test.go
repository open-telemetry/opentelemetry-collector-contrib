// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/metadata"
)

func TestNewConnector(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		latencyHistogramBuckets         []time.Duration
		expectedLatencyHistogramBuckets []float64
	}{
		{
			name:                            "simplest config (use defaults)",
			expectedLatencyHistogramBuckets: defaultLatencyHistogramBuckets,
		},
		{
			name:                            "latency histogram configured with catch-all bucket to check no additional catch-all bucket inserted",
			latencyHistogramBuckets:         []time.Duration{2 * time.Millisecond},
			expectedLatencyHistogramBuckets: []float64{0.002},
		},
		{
			name:                            "full config with no catch-all bucket and check the catch-all bucket is inserted",
			latencyHistogramBuckets:         []time.Duration{2 * time.Millisecond},
			expectedLatencyHistogramBuckets: []float64{0.002},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			factory := NewFactory()

			creationParams := connectortest.NewNopSettings(metadata.Type)
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.LatencyHistogramBuckets = tc.latencyHistogramBuckets

			// Test
			conn, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
			smc := conn.(*serviceGraphConnector)

			// Verify
			assert.NoError(t, err)
			assert.NotNil(t, smc)

			assert.Equal(t, tc.expectedLatencyHistogramBuckets, smc.reqDurationBounds)
		})
	}
}
