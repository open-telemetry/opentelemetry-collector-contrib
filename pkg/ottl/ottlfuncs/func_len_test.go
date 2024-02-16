// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Len(t *testing.T) {
	pcommonSlice := pcommon.NewSlice()
	err := pcommonSlice.FromRaw(make([]any, 5))
	if err != nil {
		t.Error(err)
	}

	pcommonMap := pcommon.NewMap()
	err = pcommonMap.FromRaw(dummyMap(5))
	if err != nil {
		t.Error(err)
	}

	pcommonValueSlice := pcommon.NewValueSlice()
	err = pcommonValueSlice.FromRaw(make([]any, 5))
	if err != nil {
		t.Error(err)
	}

	pcommonValueMap := pcommon.NewValueMap()
	err = pcommonValueMap.FromRaw(dummyMap(5))
	if err != nil {
		t.Error(err)
	}

	plogLogRecordSlice := plog.NewLogRecordSlice()
	plogLogRecordSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		plogLogRecordSlice.AppendEmpty()
	}

	plogResourceLogsSlice := plog.NewResourceLogsSlice()
	plogResourceLogsSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		plogResourceLogsSlice.AppendEmpty()
	}

	plogScopeLogsSlice := plog.NewScopeLogsSlice()
	for i := 0; i < 5; i++ {
		plogScopeLogsSlice.AppendEmpty()
	}
	plogScopeLogsSlice.EnsureCapacity(5)

	pmetricExemplarSlice := pmetric.NewExemplarSlice()
	pmetricExemplarSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricExemplarSlice.AppendEmpty()
	}

	pmetricExponentialHistogramDataPointSlice := pmetric.NewExponentialHistogramDataPointSlice()
	pmetricExponentialHistogramDataPointSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricExponentialHistogramDataPointSlice.AppendEmpty()
	}

	pmetricHistogramDataPointSlice := pmetric.NewHistogramDataPointSlice()
	pmetricHistogramDataPointSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricHistogramDataPointSlice.AppendEmpty()
	}

	pmetricMetricSlice := pmetric.NewMetricSlice()
	pmetricMetricSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricMetricSlice.AppendEmpty()
	}

	pmetricNumberDataPointSlice := pmetric.NewNumberDataPointSlice()
	pmetricNumberDataPointSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricNumberDataPointSlice.AppendEmpty()
	}

	pmetricResourceSlice := pmetric.NewResourceMetricsSlice()
	pmetricResourceSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricResourceSlice.AppendEmpty()
	}

	pmetricScopeMetricsSlice := pmetric.NewScopeMetricsSlice()
	pmetricScopeMetricsSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricScopeMetricsSlice.AppendEmpty()
	}

	pmetricSummaryDataPointSlice := pmetric.NewSummaryDataPointSlice()
	pmetricSummaryDataPointSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricSummaryDataPointSlice.AppendEmpty()
	}

	pmetricSummaryDataPointValueAtQuantileSlice := pmetric.NewSummaryDataPointValueAtQuantileSlice()
	pmetricSummaryDataPointValueAtQuantileSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		pmetricSummaryDataPointValueAtQuantileSlice.AppendEmpty()
	}

	ptraceResourceSpansSlice := ptrace.NewResourceSpansSlice()
	ptraceResourceSpansSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		ptraceResourceSpansSlice.AppendEmpty()
	}

	ptraceScopeSpansSlice := ptrace.NewScopeSpansSlice()
	ptraceScopeSpansSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		ptraceScopeSpansSlice.AppendEmpty()
	}

	ptraceSpanEventSlice := ptrace.NewSpanEventSlice()
	ptraceSpanEventSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		ptraceSpanEventSlice.AppendEmpty()
	}

	ptraceSpanLinkSlice := ptrace.NewSpanLinkSlice()
	ptraceSpanLinkSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		ptraceSpanLinkSlice.AppendEmpty()
	}

	ptraceSpanSlice := ptrace.NewSpanSlice()
	ptraceSpanSlice.EnsureCapacity(5)
	for i := 0; i < 5; i++ {
		ptraceSpanSlice.AppendEmpty()
	}

	tests := []struct {
		name     string
		value    any
		expected int64
	}{
		{
			name:     "string",
			value:    "a string",
			expected: 8,
		},
		{
			name:     "map",
			value:    dummyMap(5),
			expected: 5,
		},
		{
			name:     "string slice",
			value:    make([]string, 5),
			expected: 5,
		},
		{
			name:     "int slice",
			value:    make([]int, 5),
			expected: 5,
		},
		{
			name:     "pcommon map",
			value:    pcommonMap,
			expected: 5,
		},
		{
			name:     "pcommon slice",
			value:    pcommonSlice,
			expected: 5,
		},
		{
			name:     "pcommon value string",
			value:    pcommon.NewValueStr("a string"),
			expected: 8,
		},
		{
			name:     "pcommon value slice",
			value:    pcommonValueSlice,
			expected: 5,
		},
		{
			name:     "pcommon value map",
			value:    pcommonValueMap,
			expected: 5,
		},
		{
			name:     "plog LogRecord slice",
			value:    plogLogRecordSlice,
			expected: 5,
		},
		{
			name:     "plog ResourceLogs slice",
			value:    plogResourceLogsSlice,
			expected: 5,
		},
		{
			name:     "plog ScopeLogs slice",
			value:    plogScopeLogsSlice,
			expected: 5,
		},
		{
			name:     "pmetric Exemplar slice",
			value:    pmetricExemplarSlice,
			expected: 5,
		},
		{
			name:     "pmetric ExponentialHistogramDataPoint slice",
			value:    pmetricExponentialHistogramDataPointSlice,
			expected: 5,
		},
		{
			name:     "pmetric HistogramDataPoint slice",
			value:    pmetricHistogramDataPointSlice,
			expected: 5,
		},
		{
			name:     "pmetric Metric slice",
			value:    pmetricMetricSlice,
			expected: 5,
		},
		{
			name:     "pmetric NumberDataPoint slice",
			value:    pmetricNumberDataPointSlice,
			expected: 5,
		},
		{
			name:     "pmetric Resource slice",
			value:    pmetricResourceSlice,
			expected: 5,
		},
		{
			name:     "pmetric ScopeMetrics slice",
			value:    pmetricScopeMetricsSlice,
			expected: 5,
		},
		{
			name:     "pmetric SummaryDataPoint slice",
			value:    pmetricSummaryDataPointSlice,
			expected: 5,
		},
		{
			name:     "pmetric SummaryDataPointValueAtQuantile slice",
			value:    pmetricSummaryDataPointValueAtQuantileSlice,
			expected: 5,
		},
		{
			name:     "ptrace ResourceSpans slice",
			value:    ptraceResourceSpansSlice,
			expected: 5,
		},
		{
			name:     "ptrace ScopeSpans slice",
			value:    ptraceScopeSpansSlice,
			expected: 5,
		},
		{
			name:     "ptrace SpanEvent slice",
			value:    ptraceSpanEventSlice,
			expected: 5,
		},

		{
			name:     "ptrace SpanLink slice",
			value:    ptraceSpanLinkSlice,
			expected: 5,
		},
		{
			name:     "ptrace Span slice",
			value:    ptraceSpanSlice,
			expected: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := computeLen[any](&ottl.StandardGetSetter[any]{
				Getter: func(context context.Context, tCtx any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func dummyMap(size int) map[string]any {
	m := make(map[string]any, size)
	for i := 0; i < size; i++ {
		m[strconv.Itoa(i)] = i
	}
	return m
}

// nolint:errorlint
func Test_Len_Error(t *testing.T) {
	exprFunc := computeLen[any](&ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return 24, nil
		},
	})
	result, err := exprFunc(context.Background(), nil)
	assert.Nil(t, result)
	assert.Error(t, err)
	_, ok := err.(ottl.TypeError)
	assert.False(t, ok)
}
