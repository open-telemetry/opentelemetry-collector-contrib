package servicegraphprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestNewProcessor(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		metricsExporter                 string
		latencyHistogramBuckets         []time.Duration
		expectedLatencyHistogramBuckets []float64
	}{
		{
			name:                            "simplest config (use defaults)",
			expectedLatencyHistogramBuckets: defaultLatencyHistogramBucketsMs,
		},
		{
			name:                            "latency histogram configured with catch-all bucket to check no additional catch-all bucket inserted",
			latencyHistogramBuckets:         []time.Duration{2 * time.Millisecond, maxDuration},
			expectedLatencyHistogramBuckets: []float64{2, maxDurationMs},
		},
		{
			name:                            "full config with no catch-all bucket and check the catch-all bucket is inserted",
			latencyHistogramBuckets:         []time.Duration{2 * time.Millisecond},
			expectedLatencyHistogramBuckets: []float64{2, maxDurationMs},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			factory := NewFactory()

			creationParams := componenttest.NewNopProcessorCreateSettings()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.LatencyHistogramBuckets = tc.latencyHistogramBuckets

			// Test
			traceProcessor, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())
			smp := traceProcessor.(*processor)

			// Verify
			assert.Nil(t, err)
			assert.NotNil(t, smp)

			assert.Equal(t, tc.expectedLatencyHistogramBuckets, smp.reqDurationBounds)
		})
	}
}
