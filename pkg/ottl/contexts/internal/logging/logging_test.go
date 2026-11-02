// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zapcore"
)

func TestDataPoint_DataPointTypes(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		expected map[string]any
	}{
		{
			name: "number datapoint",
			data: func() pmetric.NumberDataPoint {
				dp := pmetric.NewNumberDataPoint()
				dp.SetIntValue(42)
				return dp
			}(),
			expected: map[string]any{"value_int": int64(42)},
		},
		{
			name: "histogram datapoint",
			data: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(2)
				return dp
			}(),
			expected: map[string]any{"count": uint64(2)},
		},
		{
			name: "exponential histogram datapoint",
			data: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(3)
				return dp
			}(),
			expected: map[string]any{"count": uint64(3)},
		},
		{
			name: "summary datapoint",
			data: func() pmetric.SummaryDataPoint {
				dp := pmetric.NewSummaryDataPoint()
				dp.SetCount(4)
				return dp
			}(),
			expected: map[string]any{"count": uint64(4)},
		},
		{
			name:     "unsupported datapoint",
			data:     struct{}{},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := zapcore.NewMapObjectEncoder()
			require.NoError(t, encoder.AddObject("datapoint", DataPoint(tt.data)))

			datapoint, ok := encoder.Fields["datapoint"].(map[string]any)
			require.True(t, ok)
			for key, expected := range tt.expected {
				assert.Equal(t, expected, datapoint[key])
			}
			if len(tt.expected) == 0 {
				assert.Empty(t, datapoint)
			}
		})
	}
}
