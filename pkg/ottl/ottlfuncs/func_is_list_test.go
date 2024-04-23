// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_IsList(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "map",
			value:    make(map[string]any, 0),
			expected: false,
		},
		{
			name:     "ValueTypeMap",
			value:    pcommon.NewValueMap(),
			expected: false,
		},
		{
			name:     "not map",
			value:    "not a map",
			expected: false,
		},
		{
			name:     "ValueTypeSlice",
			value:    pcommon.NewValueSlice(),
			expected: true,
		},
		{
			name:     "nil",
			value:    nil,
			expected: false,
		},
		{
			name:     "plog.LogRecordSlice",
			value:    plog.NewLogRecordSlice(),
			expected: true,
		},
		{
			name:     "plog.ResourceLogsSlice",
			value:    plog.NewResourceLogsSlice(),
			expected: true,
		},
		{
			name:     "plog.ScopeLogsSlice",
			value:    plog.NewScopeLogsSlice(),
			expected: true,
		},
		{
			name:     "pmetric.ExemplarSlice",
			value:    pmetric.NewExemplarSlice(),
			expected: true,
		},
		{
			name:     "pmetric.ExponentialHistogramDataPointSlice",
			value:    pmetric.NewExponentialHistogramDataPointSlice(),
			expected: true,
		},
		{
			name:     "pmetric.HistogramDataPointSlice",
			value:    pmetric.NewHistogramDataPointSlice(),
			expected: true,
		},
		{
			name:     "pmetric.MetricSlice",
			value:    pmetric.NewMetricSlice(),
			expected: true,
		},
		{
			name:     "pmetric.NumberDataPointSlice",
			value:    pmetric.NewNumberDataPointSlice(),
			expected: true,
		},
		{
			name:     "pmetric.ResourceMetricsSlice",
			value:    pmetric.NewResourceMetricsSlice(),
			expected: true,
		},
		{
			name:     "pmetric.ScopeMetricsSlice",
			value:    pmetric.NewScopeMetricsSlice(),
			expected: true,
		},
		{
			name:     "pmetric.SummaryDataPointSlice",
			value:    pmetric.NewSummaryDataPointSlice(),
			expected: true,
		},
		{
			name:     "pmetric.SummaryDataPointValueAtQuantileSlice",
			value:    pmetric.NewSummaryDataPointValueAtQuantileSlice(),
			expected: true,
		},
		{
			name:     "ptrace.ResourceSpansSlice",
			value:    ptrace.NewResourceSpansSlice(),
			expected: true,
		},
		{
			name:     "ptrace.ScopeSpansSlice",
			value:    ptrace.NewScopeSpansSlice(),
			expected: true,
		},
		{
			name:     "ptrace.SpanEventSlice",
			value:    ptrace.NewSpanEventSlice(),
			expected: true,
		},
		{
			name:     "ptrace.SpanLinkSlice",
			value:    ptrace.NewSpanLinkSlice(),
			expected: true,
		},
		{
			name:     "ptrace.SpanSlice",
			value:    ptrace.NewSpanSlice(),
			expected: true,
		},
		{
			name:     "[]string",
			value:    []string{},
			expected: true,
		},
		{
			name:     "[]bool",
			value:    []bool{},
			expected: true,
		},
		{
			name:     "[]int64",
			value:    []int64{},
			expected: true,
		},
		{
			name:     "[]float64",
			value:    []float64{},
			expected: true,
		},
		{
			name:     "[][]byte",
			value:    [][]byte{},
			expected: true,
		},
		{
			name:     "[]any",
			value:    []any{},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isList[any](&ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_IsList_Error(t *testing.T) {
	exprFunc := isList[any](&ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, ottl.TypeError("")
		},
	})
	result, err := exprFunc(context.Background(), nil)
	assert.Equal(t, false, result)
	assert.IsType(t, ottl.TypeError(""), err)
}
