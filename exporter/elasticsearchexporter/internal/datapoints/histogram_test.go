// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestHistogramValue(t *testing.T) {
	for _, tc := range []struct {
		name        string
		histogramDP pmetric.HistogramDataPoint
		expected    func(t *testing.T) pcommon.Value
	}{
		{
			name: "empty",
			histogramDP: func() pmetric.HistogramDataPoint {
				now := time.Now()
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
				now := time.Now()
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			m.SetName("test")
			tc.histogramDP.MoveTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())

			esHist := NewHistogram(m, m.Histogram().DataPoints().At(0))
			actual, err := esHist.Value()
			require.NoError(t, err)
			assert.True(t, tc.expected(t).Equal(actual))
		})
	}
}
