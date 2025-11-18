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

func TestMergeHistogramBuckets(t *testing.T) {
	tests := []struct {
		name           string
		inputBounds    []float64
		inputCounts    []uint64
		inputCount     uint64
		inputSum       float64
		bound          float64
		expectedBounds []float64
		expectedCounts []uint64

		expectNoChange bool
		expectError    bool
	}{
		{
			name:           "drop middle bucket by bound (0.5)",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			bound:          0.5,
			expectedBounds: []float64{0.1, 1.0},
			expectedCounts: []uint64{5, 11, 1},
		},
		{
			name:           "drop first finite bound (0.1)",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			bound:          0.1,
			expectedBounds: []float64{0.5, 1.0},
			expectedCounts: []uint64{13, 3, 1},
		},
		{
			name:           "drop last finite bound (1.0)",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			bound:          1.0,
			expectedBounds: []float64{0.1, 0.5},
			expectedCounts: []uint64{5, 8, 4},
		},
		{
			name:           "single finite bound collapse",
			inputBounds:    []float64{0.5},
			inputCounts:    []uint64{10, 5},
			inputCount:     15,
			inputSum:       20.0,
			bound:          0.5,
			expectedBounds: nil,
			expectedCounts: []uint64{15},
		},
		{
			name:           "drop bound (3)",
			inputBounds:    []float64{1, 2, 3, 4},
			inputCounts:    []uint64{1, 2, 3, 4, 0},
			inputCount:     10,
			inputSum:       10,
			bound:          3,
			expectedBounds: []float64{1, 2, 4},
			expectedCounts: []uint64{1, 2, 7, 0},
		},
		{
			name:           "bound not found - no change",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			bound:          2.0,
			expectedBounds: []float64{0.1, 0.5, 1.0},
			expectedCounts: []uint64{5, 8, 3, 1},

			expectNoChange: true,
		},
		{
			name:           "malformed histogram (mismatched lengths) - no change",
			inputBounds:    []float64{0.1, 0.5},
			inputCounts:    []uint64{5, 8, 3, 1, 2},
			inputCount:     19,
			inputSum:       25.5,
			bound:          0.5,
			expectedBounds: []float64{0.1, 0.5},
			expectedCounts: []uint64{5, 8, 3, 1, 2},

			expectNoChange: true,
		},
		{
			name:           "empty histogram - no change",
			inputBounds:    []float64{},
			inputCounts:    []uint64{0},
			inputCount:     0,
			inputSum:       0.0,
			bound:          0.5,
			expectedBounds: nil,
			expectedCounts: []uint64{0},

			expectNoChange: true,
		},
		{
			name:           "floating point tolerance test",
			inputBounds:    []float64{0.1, 0.5000000000001, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			bound:          0.5,
			expectedBounds: []float64{0.1, 1.0},
			expectedCounts: []uint64{5, 11, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := mergeHistogramBuckets(tt.bound)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

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

			ctx := ottldatapoint.NewTransformContext(
				dp,
				metric,
				pmetric.NewMetricSlice(),
				pcommon.NewInstrumentationScope(),
				pcommon.NewResource(),
				pmetric.NewScopeMetrics(),
				pmetric.NewResourceMetrics(),
			)

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

			if tt.expectedCounts == nil {
				assert.Nil(t, actualCounts)
			} else {
				assert.Equal(t, tt.expectedCounts, actualCounts, "Counts mismatch")
			}

			// The function does not recompute the count. We do it here just to test
			// the count is really preserved.
			var totalCount uint64
			for _, count := range actualCounts {
				totalCount += count
			}

			assert.Equal(t, tt.inputCount, totalCount, "Calculated count should be preserved")
			assert.Equal(t, tt.inputCount, dp.Count(), "Count should be preserved")
			assert.Equal(t, tt.inputSum, dp.Sum(), "Sum should be preserved")

			if len(actualBounds) > 0 && !tt.expectNoChange {
				assert.Len(t, actualCounts, len(actualBounds)+1, "Invalid histogram structure: bounds+1 != counts")
			}
		})
	}
}

func TestMergeHistogramBucketsNonHistogramDataPoint(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("gauge_metric")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(10.0)

	exprFunc, err := mergeHistogramBuckets(0.5)
	require.NoError(t, err)
	assert.NotNil(t, exprFunc)

	ctx := ottldatapoint.NewTransformContext(dp, metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())
	result, err := exprFunc(t.Context(), ctx)

	require.NoError(t, err)
	assert.Nil(t, result)

	assert.Equal(t, 10.0, dp.DoubleValue())
}

func TestNewMergeHistogramBucketsFactory(t *testing.T) {
	factory := newMergeHistogramBucketsFactory()
	assert.NotNil(t, factory)
}

func TestMergeHistogramBucketsFactoryWithInvalidArgs(t *testing.T) {
	factory := newMergeHistogramBucketsFactory()

	_, err := factory.CreateFunction(ottl.FunctionContext{}, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mergeHistogramBucketsFactory args must be of type *mergeHistogramBucketsArguments")
}
