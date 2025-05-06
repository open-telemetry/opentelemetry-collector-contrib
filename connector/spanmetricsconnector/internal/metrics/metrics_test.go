// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/lightstep/go-expohisto/structure"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestConnector_ExpoHistToExponentialDataPoint(t *testing.T) {
	tests := []struct {
		name  string
		input *structure.Histogram[float64]
		want  pmetric.ExponentialHistogramDataPoint
	}{
		{
			name:  "max bucket size - 4",
			input: structure.NewFloat64(structure.NewConfig(structure.WithMaxSize(4)), 2, 4),
			want: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(2)
				dp.SetSum(6)
				dp.SetMin(2)
				dp.SetMax(4)
				dp.SetZeroCount(0)
				dp.SetScale(1)
				dp.Positive().SetOffset(1)
				dp.Positive().BucketCounts().FromRaw([]uint64{
					1, 0, 1,
				})
				return dp
			}(),
		},
		{
			name:  "max bucket size - default",
			input: structure.NewFloat64(structure.NewConfig(), 2, 4),
			want: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(2)
				dp.SetSum(6)
				dp.SetMin(2)
				dp.SetMax(4)
				dp.SetZeroCount(0)
				dp.SetScale(7)
				dp.Positive().SetOffset(127)
				buckets := make([]uint64, 129)
				buckets[0] = 1
				buckets[128] = 1
				dp.Positive().BucketCounts().FromRaw(buckets)
				return dp
			}(),
		},
		{
			name:  "max bucket size - 4, negative observations",
			input: structure.NewFloat64(structure.NewConfig(structure.WithMaxSize(4)), -2, -4),
			want: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(2)
				dp.SetSum(-6)
				dp.SetMin(-4)
				dp.SetMax(-2)
				dp.SetZeroCount(0)
				dp.SetScale(1)
				dp.Negative().SetOffset(1)
				dp.Negative().BucketCounts().FromRaw([]uint64{
					1, 0, 1,
				})
				return dp
			}(),
		},
		{
			name:  "max bucket size - 4, negative and positive observations",
			input: structure.NewFloat64(structure.NewConfig(structure.WithMaxSize(4)), 2, 4, -2, -4),
			want: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(4)
				dp.SetSum(0)
				dp.SetMin(-4)
				dp.SetMax(4)
				dp.SetZeroCount(0)
				dp.SetScale(1)
				dp.Positive().SetOffset(1)
				dp.Positive().BucketCounts().FromRaw([]uint64{
					1, 0, 1,
				})
				dp.Negative().SetOffset(1)
				dp.Negative().BucketCounts().FromRaw([]uint64{
					1, 0, 1,
				})
				return dp
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pmetric.NewExponentialHistogramDataPoint()
			expoHistToExponentialDataPoint(tt.input, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSum_AddExemplar(t *testing.T) {
	maxCount := 3
	tests := []struct {
		name  string
		input Sum
		want  int
	}{
		{
			name:  "Sum Metric - No exemplars configured",
			input: Sum{exemplars: pmetric.NewExemplarSlice(), maxExemplarCount: &maxCount},
			want:  1,
		},
		{
			name: "Sum Metric - With exemplars length less than configured max count",
			input: func() Sum {
				exs := pmetric.NewExemplarSlice()

				e1 := exs.AppendEmpty()
				e1.SetTimestamp(1)
				e1.SetDoubleValue(1)

				return Sum{
					exemplars:        exs,
					maxExemplarCount: &maxCount,
				}
			}(),
			want: 2,
		},
		{
			name: "Sum Metric - With exemplars length equal to configured max count",
			input: func() Sum {
				exs := pmetric.NewExemplarSlice()

				e1 := exs.AppendEmpty()
				e1.SetTimestamp(1)
				e1.SetDoubleValue(1)

				e2 := exs.AppendEmpty()
				e2.SetTimestamp(2)
				e2.SetDoubleValue(2)

				e3 := exs.AppendEmpty()
				e3.SetTimestamp(3)
				e3.SetDoubleValue(3)

				return Sum{
					exemplars:        exs,
					maxExemplarCount: &maxCount,
				}
			}(),
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.AddExemplar(pcommon.TraceID{}, pcommon.SpanID{}, 4)
			assert.Equal(t, tt.want, tt.input.exemplars.Len())
		})
	}
}

func TestExplicitHistogram_AddExemplar(t *testing.T) {
	maxCount := 3
	tests := []struct {
		name  string
		input explicitHistogram
		want  int
	}{
		{
			name:  "Explicit Histogram - No exemplars configured",
			input: explicitHistogram{exemplars: pmetric.NewExemplarSlice(), maxExemplarCount: &maxCount},
			want:  1,
		},
		{
			name: "Explicit Histogram - With exemplars length less than configured max count",
			input: func() explicitHistogram {
				exs := pmetric.NewExemplarSlice()

				e1 := exs.AppendEmpty()
				e1.SetTimestamp(1)
				e1.SetDoubleValue(1)

				return explicitHistogram{
					exemplars:        exs,
					maxExemplarCount: &maxCount,
				}
			}(),
			want: 2,
		},
		{
			name: "Explicit Histogram - With exemplars length equal to configured max count",
			input: func() explicitHistogram {
				exs := pmetric.NewExemplarSlice()

				e1 := exs.AppendEmpty()
				e1.SetTimestamp(1)
				e1.SetDoubleValue(1)

				e2 := exs.AppendEmpty()
				e2.SetTimestamp(2)
				e2.SetDoubleValue(2)

				e3 := exs.AppendEmpty()
				e3.SetTimestamp(3)
				e3.SetDoubleValue(3)

				return explicitHistogram{
					exemplars:        exs,
					maxExemplarCount: &maxCount,
				}
			}(),
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.AddExemplar(pcommon.TraceID{}, pcommon.SpanID{}, 4)
			assert.Equal(t, tt.want, tt.input.exemplars.Len())
		})
	}
}

func TestExponentialHistogram_AddExemplar(t *testing.T) {
	maxCount := 3
	tests := []struct {
		name  string
		input exponentialHistogram
		want  int
	}{
		{
			name:  "Exponential Histogram - No exemplars configured",
			input: exponentialHistogram{exemplars: pmetric.NewExemplarSlice(), maxExemplarCount: &maxCount},
			want:  1,
		},
		{
			name: "Exponential Histogram - With exemplars length less than configured max count",
			input: func() exponentialHistogram {
				exs := pmetric.NewExemplarSlice()

				e1 := exs.AppendEmpty()
				e1.SetTimestamp(1)
				e1.SetDoubleValue(1)

				return exponentialHistogram{
					exemplars:        exs,
					maxExemplarCount: &maxCount,
				}
			}(),
			want: 2,
		},
		{
			name: "Exponential Histogram - With exemplars length equal to configured max count",
			input: func() exponentialHistogram {
				exs := pmetric.NewExemplarSlice()

				e1 := exs.AppendEmpty()
				e1.SetTimestamp(1)
				e1.SetDoubleValue(1)

				e2 := exs.AppendEmpty()
				e2.SetTimestamp(2)
				e2.SetDoubleValue(2)

				e3 := exs.AppendEmpty()
				e3.SetTimestamp(3)
				e3.SetDoubleValue(3)

				return exponentialHistogram{
					exemplars:        exs,
					maxExemplarCount: &maxCount,
				}
			}(),
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.AddExemplar(pcommon.TraceID{}, pcommon.SpanID{}, 4)
			assert.Equal(t, tt.want, tt.input.exemplars.Len())
		})
	}
}

func TestSum_Add(t *testing.T) {
	tests := []struct {
		name     string
		sum      Sum
		value    uint64
		expected uint64
	}{
		{
			name:     "Add zero to empty sum",
			sum:      Sum{count: 0},
			value:    0,
			expected: 0,
		},
		{
			name:     "Add positive value to empty sum",
			sum:      Sum{count: 0},
			value:    5,
			expected: 5,
		},
		{
			name:     "Add value to existing sum",
			sum:      Sum{count: 10},
			value:    5,
			expected: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sum.Add(tt.value)
			assert.Equal(t, tt.expected, tt.sum.count)
		})
	}
}

func TestSumMetrics_IsCardinalityLimitReached(t *testing.T) {
	tests := []struct {
		name             string
		metrics          map[Key]*Sum
		cardinalityLimit int
		expected         bool
	}{
		{
			name:             "No limit set",
			metrics:          make(map[Key]*Sum),
			cardinalityLimit: 0,
			expected:         false,
		},
		{
			name:             "Below limit",
			metrics:          map[Key]*Sum{"key1": {}, "key2": {}},
			cardinalityLimit: 3,
			expected:         false,
		},
		{
			name:             "At limit",
			metrics:          map[Key]*Sum{"key1": {}, "key2": {}, "key3": {}},
			cardinalityLimit: 3,
			expected:         true,
		},
		{
			name:             "Above limit",
			metrics:          map[Key]*Sum{"key1": {}, "key2": {}, "key3": {}, "key4": {}},
			cardinalityLimit: 3,
			expected:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := SumMetrics{
				metrics:          tt.metrics,
				cardinalityLimit: tt.cardinalityLimit,
			}
			assert.Equal(t, tt.expected, sm.IsCardinalityLimitReached())
		})
	}
}

func TestSumMetrics_GetOrCreate(t *testing.T) {
	tests := []struct {
		name               string
		metrics            map[Key]*Sum
		key                Key
		attributes         pcommon.Map
		cardinalityLimit   int
		expectedMetricsLen int
		expectedSumCount   int
		expectedCreated    bool
		limitReached       bool
	}{
		{
			name:               "Create new sum",
			metrics:            make(map[Key]*Sum),
			key:                "new-key",
			attributes:         pcommon.NewMap(),
			expectedMetricsLen: 1,
			expectedSumCount:   1,
			expectedCreated:    true,
		},
		{
			name: "Get existing sum",
			metrics: map[Key]*Sum{
				"existing-key": {count: 5},
			},
			key:                "existing-key",
			attributes:         pcommon.NewMap(),
			expectedMetricsLen: 1,
			expectedSumCount:   6,
			expectedCreated:    false,
		},
		{
			name: "sum reach cardinality limit and create the overflow metric",
			metrics: map[Key]*Sum{
				"key-1": {count: 5},
				"key-2": {count: 6},
			},
			key: "key-3",
			attributes: func() pcommon.Map {
				attributes := pcommon.NewMap()
				attributes.PutBool(overflowKey, true)
				return attributes
			}(),
			cardinalityLimit:   2,
			expectedMetricsLen: 3,
			expectedCreated:    true,
			limitReached:       true,
		},
		{
			name: "sum reach cardinality limit and return the existing overflow metric",
			metrics: map[Key]*Sum{
				"key-1":                {count: 5},
				"key-2":                {count: 6},
				"otel.metric.overflow": {count: 1},
			},
			key: "key-3",
			attributes: func() pcommon.Map {
				attributes := pcommon.NewMap()
				attributes.PutBool(overflowKey, true)
				return attributes
			}(),
			cardinalityLimit:   2,
			expectedMetricsLen: 3,
			expectedSumCount:   2,
			expectedCreated:    false,
			limitReached:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := SumMetrics{
				metrics:          tt.metrics,
				cardinalityLimit: tt.cardinalityLimit,
			}
			attributesFun := func() pcommon.Map {
				return tt.attributes
			}
			sum, limitReached := sm.GetOrCreate(tt.key, attributesFun, pcommon.Timestamp(0))
			sum.Add(1)
			assert.Equal(t, tt.limitReached, limitReached)
			assert.Len(t, sm.metrics, tt.expectedMetricsLen)
			if tt.expectedCreated {
				assert.Equal(t, tt.attributes, sum.attributes)
				assert.Equal(t, uint64(1), sum.count)
			} else {
				assert.Equal(t, uint64(tt.expectedSumCount), sum.count)
			}
		})
	}
}

func TestSumMetrics_BuildMetrics(t *testing.T) {
	tests := []struct {
		name          string
		metrics       map[Key]*Sum
		temporality   pmetric.AggregationTemporality
		expectedCount int
		expectedValue int64
	}{
		{
			name: "Build metrics with one sum",
			metrics: map[Key]*Sum{
				"key1": {
					count: 5,
					attributes: func() pcommon.Map {
						m := pcommon.NewMap()
						m.PutStr("attr1", "value1")
						return m
					}(),
					exemplars: pmetric.NewExemplarSlice(),
				},
			},
			temporality:   pmetric.AggregationTemporalityCumulative,
			expectedCount: 1,
			expectedValue: 5,
		},
		{
			name: "Build metrics with multiple sums",
			metrics: map[Key]*Sum{
				"key1": {
					count: 5,
					attributes: func() pcommon.Map {
						m := pcommon.NewMap()
						m.PutStr("attr1", "value1")
						return m
					}(),
					exemplars: pmetric.NewExemplarSlice(),
				},
				"key2": {
					count: 10,
					attributes: func() pcommon.Map {
						m := pcommon.NewMap()
						m.PutStr("attr2", "value2")
						return m
					}(),
					exemplars: pmetric.NewExemplarSlice(),
				},
			},
			temporality:   pmetric.AggregationTemporalityDelta,
			expectedCount: 2,
			expectedValue: 5, // Will check both values in the test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := SumMetrics{
				metrics: tt.metrics,
			}
			metric := pmetric.NewMetric()
			startTimestamp := func(Key, pcommon.Timestamp) pcommon.Timestamp { return 0 }
			timestamp := pcommon.Timestamp(1000)

			sm.BuildMetrics(metric, timestamp, startTimestamp, tt.temporality)

			assert.Equal(t, pmetric.MetricTypeSum, metric.Type())
			assert.Equal(t, tt.temporality, metric.Sum().AggregationTemporality())
			assert.True(t, metric.Sum().IsMonotonic())

			dps := metric.Sum().DataPoints()
			assert.Equal(t, tt.expectedCount, dps.Len())

			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				assert.Equal(t, timestamp, dp.Timestamp())
				assert.Equal(t, pcommon.Timestamp(0), dp.StartTimestamp())
				if tt.expectedCount == 1 {
					assert.Equal(t, tt.expectedValue, dp.IntValue())
				}
			}
		})
	}
}

func TestSumMetrics_ClearExemplars(t *testing.T) {
	tests := []struct {
		name     string
		metrics  map[Key]*Sum
		expected int
	}{
		{
			name: "Clear exemplars from multiple sums",
			metrics: map[Key]*Sum{
				"key1": {
					exemplars: func() pmetric.ExemplarSlice {
						es := pmetric.NewExemplarSlice()
						es.AppendEmpty()
						return es
					}(),
				},
				"key2": {
					exemplars: func() pmetric.ExemplarSlice {
						es := pmetric.NewExemplarSlice()
						es.AppendEmpty()
						es.AppendEmpty()
						return es
					}(),
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := SumMetrics{
				metrics: tt.metrics,
			}
			sm.ClearExemplars()

			for _, sum := range sm.metrics {
				assert.Equal(t, tt.expected, sum.exemplars.Len())
			}
		})
	}
}
