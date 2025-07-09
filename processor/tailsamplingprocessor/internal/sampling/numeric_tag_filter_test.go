// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestNumericTagFilter(t *testing.T) {
	empty := map[string]any{}
	minVal := int64(math.MinInt32)
	maxVal := int64(math.MaxInt32)
	filter := NewNumericAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", &minVal, &maxVal, false)

	resAttr := map[string]any{}
	resAttr["example"] = 8

	cases := []struct {
		Desc     string
		Trace    *TraceData
		Decision Decision
	}{
		{
			Desc:     "nonmatching span attribute",
			Trace:    newTraceIntAttrs(empty, "non_matching", math.MinInt32),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute at the lower limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MinInt32),
			Decision: Sampled,
		},
		{
			Desc:     "resource attribute at the lower limit",
			Trace:    newTraceIntAttrs(map[string]any{"example": math.MinInt32}, "non_matching", math.MinInt32),
			Decision: Sampled,
		},
		{
			Desc:     "span attribute at the upper limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MaxInt32),
			Decision: Sampled,
		},
		{
			Desc:     "resource attribute at the upper limit",
			Trace:    newTraceIntAttrs(map[string]any{"example": math.MaxInt32}, "non_matching", math.MaxInt),
			Decision: Sampled,
		},
		{
			Desc:     "span attribute below min limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MinInt32-1),
			Decision: NotSampled,
		},
		{
			Desc:     "resource attribute below min limit",
			Trace:    newTraceIntAttrs(map[string]any{"example": math.MinInt32 - 1}, "non_matching", math.MinInt32),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute above max limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MaxInt32+1),
			Decision: NotSampled,
		},
		{
			Desc:     "resource attribute above max limit",
			Trace:    newTraceIntAttrs(map[string]any{"example": math.MaxInt32 + 1}, "non_matching", math.MaxInt32),
			Decision: NotSampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			u, _ := uuid.NewRandom()
			decision, err := filter.Evaluate(context.Background(), pcommon.TraceID(u), c.Trace)
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func TestNumericTagFilterInverted(t *testing.T) {
	empty := map[string]any{}
	minVal := int64(math.MinInt32)
	maxVal := int64(math.MaxInt32)
	filter := NewNumericAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", &minVal, &maxVal, true)

	resAttr := map[string]any{}
	resAttr["example"] = 8

	cases := []struct {
		Desc                  string
		Trace                 *TraceData
		Decision              Decision
		DisableInvertDecision bool
	}{
		{
			Desc:     "nonmatching span attribute",
			Trace:    newTraceIntAttrs(empty, "non_matching", math.MinInt32),
			Decision: InvertSampled,
		},
		{
			Desc:     "span attribute at the lower limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MinInt32),
			Decision: InvertNotSampled,
		},
		{
			Desc:     "resource attribute at the lower limit",
			Trace:    newTraceIntAttrs(map[string]any{"example": math.MinInt32}, "non_matching", math.MinInt32),
			Decision: InvertNotSampled,
		},
		{
			Desc:     "span attribute at the upper limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MaxInt32),
			Decision: InvertNotSampled,
		},
		{
			Desc:     "resource attribute at the upper limit",
			Trace:    newTraceIntAttrs(map[string]any{"example": math.MaxInt32}, "non_matching", math.MaxInt32),
			Decision: InvertNotSampled,
		},
		{
			Desc:     "span attribute below min limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MinInt32-1),
			Decision: InvertSampled,
		},
		{
			Desc:     "resource attribute below min limit",
			Trace:    newTraceIntAttrs(map[string]any{"example": math.MinInt32 - 1}, "non_matching", math.MinInt32),
			Decision: InvertSampled,
		},
		{
			Desc:     "span attribute above max limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MaxInt32+1),
			Decision: InvertSampled,
		},
		{
			Desc:     "resource attribute above max limit",
			Trace:    newTraceIntAttrs(map[string]any{"example": math.MaxInt32 + 1}, "non_matching", math.MaxInt32+1),
			Decision: InvertSampled,
		},
		{
			Desc:                  "nonmatching span attribute with DisableInvertDecision",
			Trace:                 newTraceIntAttrs(empty, "non_matching", math.MinInt32),
			Decision:              Sampled,
			DisableInvertDecision: true,
		},
		{
			Desc:                  "span attribute at the lower limit with DisableInvertDecision",
			Trace:                 newTraceIntAttrs(empty, "example", math.MinInt32),
			Decision:              NotSampled,
			DisableInvertDecision: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			if c.DisableInvertDecision {
				err := featuregate.GlobalRegistry().Set("processor.tailsamplingprocessor.disableinvertdecisions", true)
				assert.NoError(t, err)
				defer func() {
					err := featuregate.GlobalRegistry().Set("processor.tailsamplingprocessor.disableinvertdecisions", false)
					assert.NoError(t, err)
				}()
			}
			u, _ := uuid.NewRandom()
			decision, err := filter.Evaluate(context.Background(), pcommon.TraceID(u), c.Trace)
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func TestNumericTagFilterOptionalBounds(t *testing.T) {
	tests := []struct {
		name        string
		min         *int64
		max         *int64
		value       int64
		invertMatch bool
		want        Decision
	}{
		{
			name:  "only min set - value above min",
			min:   ptr(int64(100)),
			max:   nil,
			value: 200,
			want:  Sampled,
		},
		{
			name:  "only min set - value below min",
			min:   ptr(int64(100)),
			max:   nil,
			value: 50,
			want:  NotSampled,
		},
		{
			name:  "only max set - value below max",
			min:   nil,
			max:   ptr(int64(100)),
			value: 50,
			want:  Sampled,
		},
		{
			name:  "only max set - value above max",
			min:   nil,
			max:   ptr(int64(100)),
			value: 200,
			want:  NotSampled,
		},
		{
			name:  "both set - value in range",
			min:   ptr(int64(100)),
			max:   ptr(int64(200)),
			value: 150,
			want:  Sampled,
		},
		{
			name:  "both set - value out of range",
			min:   ptr(int64(100)),
			max:   ptr(int64(200)),
			value: 50,
			want:  NotSampled,
		},
		{
			name:        "inverted match - only min set - value above min",
			min:         ptr(int64(100)),
			max:         nil,
			value:       200,
			invertMatch: true,
			want:        InvertNotSampled,
		},
		{
			name:        "inverted match - only max set - value below max",
			min:         nil,
			max:         ptr(int64(100)),
			value:       50,
			invertMatch: true,
			want:        InvertNotSampled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewNumericAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", tt.min, tt.max, tt.invertMatch)
			require.NotNil(t, filter, "filter should not be nil")

			trace := newTraceIntAttrs(map[string]any{}, "example", tt.value)
			decision, err := filter.Evaluate(context.Background(), pcommon.TraceID{}, trace)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, decision)
		})
	}
}

func TestNumericTagFilterNilBounds(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	filter := NewNumericAttributeFilter(settings, "example", nil, nil, false)
	assert.Nil(t, filter, "filter should be nil when both bounds are nil")

	// Test that the filter is created successfully when at least one bound is set
	minBound := int64(100)
	filter = NewNumericAttributeFilter(settings, "example", &minBound, nil, false)
	assert.NotNil(t, filter, "filter should not be nil when min is set")

	maxBound := int64(200)
	filter = NewNumericAttributeFilter(settings, "example", nil, &maxBound, false)
	assert.NotNil(t, filter, "filter should not be nil when max is set")

	filter = NewNumericAttributeFilter(settings, "example", &minBound, &maxBound, false)
	assert.NotNil(t, filter, "filter should not be nil when both bounds are set")
}

// helper function to create int64 pointer
func ptr(i int64) *int64 {
	return &i
}

func newTraceIntAttrs(nodeAttrs map[string]any, spanAttrKey string, spanAttrValue int64) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	//nolint:errcheck
	rs.Resource().Attributes().FromRaw(nodeAttrs)
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutInt(spanAttrKey, spanAttrValue)
	return &TraceData{
		ReceivedBatches: traces,
	}
}
