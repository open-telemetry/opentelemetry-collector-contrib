// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewProcessor(t *testing.T) {
	defaultMethod := "GET"
	defaultMethodValue := pcommon.NewValueStr(defaultMethod)
	for _, tc := range []struct {
		name                        string
		metricsExporter             string
		latencyHistogramBuckets     []time.Duration
		dimensions                  []Dimension
		wantLatencyHistogramBuckets []float64
		wantDimensions              []dimension
	}{
		{
			name:                        "simplest config (use defaults)",
			wantLatencyHistogramBuckets: defaultLatencyHistogramBucketsMs,
		},
		{
			name:                    "1 configured latency histogram bucket should result in 1 explicit latency bucket (+1 implicit +Inf bucket)",
			latencyHistogramBuckets: []time.Duration{2 * time.Millisecond},
			dimensions: []Dimension{
				{Name: "http.method", Default: &defaultMethod},
				{Name: "http.status_code"},
			},
			wantLatencyHistogramBuckets: []float64{2},
			wantDimensions: []dimension{
				{name: "http.method", value: &defaultMethodValue},
				{"http.status_code", nil},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			factory := NewFactory()

			creationParams := processortest.NewNopCreateSettings()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.LatencyHistogramBuckets = tc.latencyHistogramBuckets
			cfg.Dimensions = tc.dimensions

			// Test
			traceProcessor, err := factory.CreateTracesProcessor(context.Background(), creationParams, cfg, consumertest.NewNop())
			smp := traceProcessor.(*processorImp)

			// Verify
			assert.Nil(t, err)
			assert.NotNil(t, smp)

			assert.Equal(t, tc.wantLatencyHistogramBuckets, smp.latencyBounds)
			assert.Equal(t, tc.wantDimensions, smp.dimensions)
		})
	}
}
