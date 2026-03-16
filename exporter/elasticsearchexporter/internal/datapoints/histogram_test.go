// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestHistogramValue(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name        string
		histogramDP pmetric.HistogramDataPoint
		mappingMode string
		expectedErr error
		expected    func(t *testing.T) pcommon.Value
	}{
		{
			name: "empty",
			histogramDP: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				return dp
			}(),
			expected: func(t *testing.T) pcommon.Value {
				t.Helper()
				v := pcommon.NewValueMap()
				require.NoError(t, v.FromRaw(map[string]any{
					"counts": []any{},
					"values": []any{},
				}))
				return v
			},
		},
		{
			name: "required_fields_only",
			histogramDP: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.SetCount(100)
				dp.SetSum(1000)
				return dp
			}(),
			expected: func(t *testing.T) pcommon.Value {
				t.Helper()
				v := pcommon.NewValueMap()
				require.NoError(t, v.FromRaw(map[string]any{
					"counts": []any{100},
					"values": []any{10.0},
				}))
				return v
			},
		},
		{
			name: "default_midpoint_with_explicit_bounds",
			histogramDP: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.ExplicitBounds().FromRaw([]float64{10, 20, 30})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
				return dp
			}(),
			expected: func(t *testing.T) pcommon.Value {
				t.Helper()
				v := pcommon.NewValueMap()
				require.NoError(t, v.FromRaw(map[string]any{
					"counts": []any{1, 2, 3, 4},
					"values": []any{5.0, 15.0, 25.0, 30.0},
				}))
				return v
			},
		},
		{
			name: "preserve_intake_histogram_values",
			histogramDP: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.SetCount(50)
				dp.SetSum(3350)
				dp.ExplicitBounds().FromRaw([]float64{10, 50, 100, 200})
				dp.BucketCounts().FromRaw([]uint64{10, 25, 10, 5})
				dp.Attributes().PutStr("processor.event", "metric")
				return dp
			}(),
			mappingMode: "ecs",
			expected: func(t *testing.T) pcommon.Value {
				t.Helper()
				v := pcommon.NewValueMap()
				require.NoError(t, v.FromRaw(map[string]any{
					"counts": []any{10, 25, 10, 5},
					"values": []any{10.0, 50.0, 100.0, 200.0},
				}))
				return v
			},
		},
		{
			name: "non_ecs_mode_falls_through_to_standard_validation",
			histogramDP: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.ExplicitBounds().FromRaw([]float64{10, 50, 100, 200})
				dp.BucketCounts().FromRaw([]uint64{10, 25, 10, 5})
				dp.Attributes().PutStr("processor.event", "metric")
				return dp
			}(),
			mappingMode: "otel",
			expectedErr: fmt.Errorf("invalid histogram data point %q", "test"),
		},
		{
			name: "ecs_mode_missing_processor_event_falls_through",
			histogramDP: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.ExplicitBounds().FromRaw([]float64{10, 50, 100, 200})
				dp.BucketCounts().FromRaw([]uint64{10, 25, 10, 5})
				return dp
			}(),
			mappingMode: "ecs",
			expectedErr: fmt.Errorf("invalid histogram data point %q", "test"),
		},
		{
			name: "ecs_mode_wrong_processor_event_falls_through",
			histogramDP: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.ExplicitBounds().FromRaw([]float64{10, 50, 100, 200})
				dp.BucketCounts().FromRaw([]uint64{10, 25, 10, 5})
				dp.Attributes().PutStr("processor.event", "transaction")
				return dp
			}(),
			mappingMode: "ecs",
			expectedErr: fmt.Errorf("invalid histogram data point %q", "test"),
		},
		{
			name: "intake_histogram_values_invalid_shape",
			histogramDP: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.ExplicitBounds().FromRaw([]float64{10, 50})
				dp.BucketCounts().FromRaw([]uint64{10, 25, 10, 5})
				dp.Attributes().PutStr("processor.event", "metric")
				return dp
			}(),
			mappingMode: "ecs",
			expectedErr: fmt.Errorf("invalid histogram data point %q", "test"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			m.SetName("test")
			tc.histogramDP.MoveTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())

			esHist := NewHistogram(m, m.Histogram().DataPoints().At(0), tc.mappingMode)
			actual, err := esHist.Value()
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
				return
			}
			require.NoError(t, err)
			assert.True(t, tc.expected(t).Equal(actual))
		})
	}
}
