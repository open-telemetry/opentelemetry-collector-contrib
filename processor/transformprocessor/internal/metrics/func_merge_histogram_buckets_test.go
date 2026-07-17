// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
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
		targetValue    any
		method         ottl.Optional[string]
		expectedBounds []float64
		expectedCounts []uint64

		expectNoChange     bool
		expectError        bool
		expectProcessError bool
	}{
		{
			name:           "drop middle bucket by bound (0.5)",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			targetValue:    0.5,
			expectedBounds: []float64{0.1, 1.0},
			expectedCounts: []uint64{5, 11, 1},
		},
		{
			name:           "drop first finite bound (0.1)",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			targetValue:    0.1,
			expectedBounds: []float64{0.5, 1.0},
			expectedCounts: []uint64{13, 3, 1},
		},
		{
			name:           "drop last finite bound (1.0)",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			targetValue:    1.0,
			expectedBounds: []float64{0.1, 0.5},
			expectedCounts: []uint64{5, 8, 4},
		},
		{
			name:           "single finite bound collapse",
			inputBounds:    []float64{0.5},
			inputCounts:    []uint64{10, 5},
			inputCount:     15,
			inputSum:       20.0,
			targetValue:    0.5,
			expectedBounds: nil,
			expectedCounts: []uint64{15},
		},
		{
			name:           "drop bound (3)",
			inputBounds:    []float64{1, 2, 3, 4},
			inputCounts:    []uint64{1, 2, 3, 4, 0},
			inputCount:     10,
			inputSum:       10,
			targetValue:    3.0,
			expectedBounds: []float64{1, 2, 4},
			expectedCounts: []uint64{1, 2, 7, 0},
		},
		{
			name:           "bound not found - no change",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			targetValue:    2.0,
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
			targetValue:    0.5,
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
			targetValue:    0.5,
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
			targetValue:    0.5,
			expectedBounds: []float64{0.1, 1.0},
			expectedCounts: []uint64{5, 11, 1},
		},
		{
			name:           "limit buckets by uniform compaction",
			inputBounds:    []float64{0.1, 0.2, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
			inputCounts:    []uint64{80, 4, 6, 120, 3, 2, 40, 10, 1},
			inputCount:     266,
			inputSum:       512.5,
			targetValue:    int64(5),
			method:         ottl.NewTestingOptional(mergeHistogramBucketsMethodLimitBuckets),
			expectedBounds: []float64{0.2, 1.0, 5.0, 30.0},
			expectedCounts: []uint64{84, 126, 5, 50, 1},
		},
		{
			name:           "limit buckets uses smallest divisor that stays within limit",
			inputBounds:    []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			inputCounts:    []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			inputCount:     55,
			inputSum:       385,
			targetValue:    int64(4),
			method:         ottl.NewTestingOptional(mergeHistogramBucketsMethodLimitBuckets),
			expectedBounds: []float64{3, 6, 9},
			expectedCounts: []uint64{6, 15, 24, 10},
		},
		{
			name:           "limit buckets single compaction pass may reduce below limit",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			targetValue:    int64(3),
			method:         ottl.NewTestingOptional(mergeHistogramBucketsMethodLimitBuckets),
			expectedBounds: []float64{0.5},
			expectedCounts: []uint64{13, 4},
		},
		{
			name:           "limit buckets collapse to one bucket",
			inputBounds:    []float64{1, 2},
			inputCounts:    []uint64{1, 2, 3},
			inputCount:     6,
			inputSum:       12,
			targetValue:    int64(1),
			method:         ottl.NewTestingOptional(mergeHistogramBucketsMethodLimitBuckets),
			expectedBounds: nil,
			expectedCounts: []uint64{6},
		},
		{
			name:           "limit buckets already within limit",
			inputBounds:    []float64{0.1, 0.5, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			targetValue:    int64(4),
			method:         ottl.NewTestingOptional(mergeHistogramBucketsMethodLimitBuckets),
			expectedBounds: []float64{0.1, 0.5, 1.0},
			expectedCounts: []uint64{5, 8, 3, 1},
			expectNoChange: true,
		},
		{
			name:           "limit buckets unordered bounds do not change",
			inputBounds:    []float64{0.5, 0.1, 1.0},
			inputCounts:    []uint64{5, 8, 3, 1},
			inputCount:     17,
			inputSum:       25.5,
			targetValue:    int64(2),
			method:         ottl.NewTestingOptional(mergeHistogramBucketsMethodLimitBuckets),
			expectedBounds: []float64{0.5, 0.1, 1.0},
			expectedCounts: []uint64{5, 8, 3, 1},
			expectNoChange: true,
		},
		{
			name:               "limit buckets rejects fractional target value",
			inputBounds:        []float64{0.1, 0.5, 1.0},
			inputCounts:        []uint64{5, 8, 3, 1},
			inputCount:         17,
			inputSum:           25.5,
			targetValue:        2.5,
			method:             ottl.NewTestingOptional(mergeHistogramBucketsMethodLimitBuckets),
			expectProcessError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := mergeHistogramBuckets(floatLikeGetter(tt.targetValue), tt.method)

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

			ctx := ottldatapoint.NewTransformContextPtr(pmetric.NewResourceMetrics(), pmetric.NewScopeMetrics(), metric, dp)
			defer ctx.Close()

			result, err := exprFunc(t.Context(), ctx)

			if tt.expectProcessError {
				assert.Error(t, err)
				return
			}

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

	exprFunc, err := mergeHistogramBuckets(floatLikeGetter(0.5), ottl.Optional[string]{})
	require.NoError(t, err)
	assert.NotNil(t, exprFunc)

	ctx := ottldatapoint.NewTransformContextPtr(pmetric.NewResourceMetrics(), pmetric.NewScopeMetrics(), metric, dp)
	defer ctx.Close()
	result, err := exprFunc(t.Context(), ctx)

	require.NoError(t, err)
	assert.Nil(t, result)

	assert.Equal(t, 10.0, dp.DoubleValue())
}

func TestMergeHistogramBucketsInvalidMethod(t *testing.T) {
	_, err := mergeHistogramBuckets(floatLikeGetter(0.5), ottl.NewTestingOptional("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unsupported method "invalid"`)
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

func floatLikeGetter(value any) ottl.FloatLikeGetter[*ottldatapoint.TransformContext] {
	return ottl.StandardFloatLikeGetter[*ottldatapoint.TransformContext]{
		Getter: func(context.Context, *ottldatapoint.TransformContext) (any, error) {
			return value, nil
		},
	}
}
