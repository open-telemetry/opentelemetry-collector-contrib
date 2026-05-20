// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

func TestLimitHistogramBuckets(t *testing.T) {
	tests := []struct {
		name           string
		inputBounds    []float64
		inputCounts    []uint64
		inputCount     uint64
		inputSum       float64
		maxBuckets     int64
		expectedBounds []float64
		expectedCounts []uint64
		expectNoChange bool
	}{
		{
			name:           "limit by uniform compaction, not counts",
			inputBounds:    []float64{0.1, 0.2, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
			inputCounts:    []uint64{80, 4, 6, 120, 3, 2, 40, 10, 1},
			inputCount:     266,
			inputSum:       512.5,
			maxBuckets:     5,
			expectedBounds: []float64{0.2, 1.0, 5.0, 30.0},
			expectedCounts: []uint64{84, 126, 5, 50, 1},
		},
		{
			name:           "single compaction pass may reduce below limit",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			maxBuckets:     3,
			expectedBounds: []float64{0.5},
			expectedCounts: []uint64{13, 4},
		},
		{
			name:           "collapse to one bucket",
			inputBounds:    []float64{1, 2},
			inputCounts:    []uint64{1, 2, 3},
			inputCount:     6,
			inputSum:       12,
			maxBuckets:     1,
			expectedBounds: nil,
			expectedCounts: []uint64{6},
		},
		{
			name:           "already within limit",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			maxBuckets:     4,
			expectedBounds: []float64{0.1, 0.5, 1.0},
			expectedCounts: []uint64{5, 8, 3, 1},
			expectNoChange: true,
		},
		{
			name:           "compacts evenly when only one bucket over limit",
			inputBounds:    []float64{1, 2, 3, 4},
			inputCounts:    []uint64{1, 2, 3, 4, 5},
			inputCount:     15,
			inputSum:       20,
			maxBuckets:     4,
			expectedBounds: []float64{2, 4},
			expectedCounts: []uint64{3, 7, 5},
		},
		{
			name:           "malformed histogram does not change",
			inputBounds:    []float64{0.1, 0.5},
			inputCounts:    []uint64{5, 8, 3, 1, 2},
			inputCount:     19,
			inputSum:       25.5,
			maxBuckets:     2,
			expectedBounds: []float64{0.1, 0.5},
			expectedCounts: []uint64{5, 8, 3, 1, 2},
			expectNoChange: true,
		},
		{
			name:           "unsorted bounds do not change",
			inputBounds:    []float64{0.5, 0.1, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			maxBuckets:     2,
			expectedBounds: []float64{0.5, 0.1, 1.0},
			expectedCounts: []uint64{5, 8, 3, 1},
			expectNoChange: true,
		},
		{
			name:           "single bucket histogram does not change",
			inputBounds:    []float64{},
			inputCounts:    []uint64{0},
			inputCount:     0,
			inputSum:       0,
			maxBuckets:     1,
			expectedBounds: nil,
			expectedCounts: []uint64{0},
			expectNoChange: true,
		},
		{
			name:           "empty bucket counts do not change",
			inputBounds:    nil,
			inputCounts:    nil,
			inputCount:     0,
			inputSum:       0,
			maxBuckets:     1,
			expectedBounds: nil,
			expectedCounts: nil,
			expectNoChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := limitHistogramBuckets(tt.maxBuckets)
			require.NoError(t, err)
			assert.NotNil(t, exprFunc)

			metric := pmetric.NewMetric()
			metric.SetName("test_histogram")
			hist := metric.SetEmptyHistogram()
			dp := hist.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetCount(tt.inputCount)
			dp.SetSum(tt.inputSum)
			dp.BucketCounts().FromRaw(tt.inputCounts)
			dp.ExplicitBounds().FromRaw(tt.inputBounds)

			ctx := ottldatapoint.NewTransformContextPtr(pmetric.NewResourceMetrics(), pmetric.NewScopeMetrics(), metric, dp)
			defer ctx.Close()

			result, err := exprFunc(t.Context(), ctx)

			require.NoError(t, err)
			assert.Nil(t, result)

			actualBounds := dp.ExplicitBounds().AsRaw()
			actualCounts := dp.BucketCounts().AsRaw()
			if tt.expectedBounds == nil {
				assert.Nil(t, actualBounds)
			} else {
				assert.Equal(t, tt.expectedBounds, actualBounds, "Bounds mismatch")
			}

			assert.Equal(t, tt.expectedCounts, actualCounts, "Counts mismatch")

			var totalCount uint64
			for _, count := range actualCounts {
				totalCount += count
			}

			assert.Equal(t, tt.inputCount, totalCount, "Calculated count should be preserved")
			assert.Equal(t, tt.inputCount, dp.Count(), "Count should be preserved")
			assert.Equal(t, tt.inputSum, dp.Sum(), "Sum should be preserved")
			if !tt.expectNoChange {
				assert.LessOrEqual(t, len(actualCounts), int(tt.maxBuckets))
			}
			if len(actualCounts) > 0 && !tt.expectNoChange {
				assert.Len(t, actualCounts, len(actualBounds)+1, "Invalid histogram structure: bounds+1 != counts")
			}
		})
	}
}

func TestLimitHistogramBucketsInvalidMaxBuckets(t *testing.T) {
	_, err := limitHistogramBuckets(0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_buckets must be greater than 0")
}

func TestLimitHistogramBucketsNonHistogramDataPoint(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("gauge_metric")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(10.0)

	exprFunc, err := limitHistogramBuckets(1)
	require.NoError(t, err)
	assert.NotNil(t, exprFunc)

	ctx := ottldatapoint.NewTransformContextPtr(pmetric.NewResourceMetrics(), pmetric.NewScopeMetrics(), metric, dp)
	defer ctx.Close()
	result, err := exprFunc(t.Context(), ctx)

	require.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 10.0, dp.DoubleValue())
}

func TestNewLimitHistogramBucketsFactory(t *testing.T) {
	factory := newLimitHistogramBucketsFactory()
	assert.NotNil(t, factory)
}

func TestLimitHistogramBucketsFactoryWithInvalidArgs(t *testing.T) {
	factory := newLimitHistogramBucketsFactory()

	_, err := factory.CreateFunction(ottl.FunctionContext{}, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "limitHistogramBucketsFactory args must be of type *limitHistogramBucketsArguments")
}
