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
