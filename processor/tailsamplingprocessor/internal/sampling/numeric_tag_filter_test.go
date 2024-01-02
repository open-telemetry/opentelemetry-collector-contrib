// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestNumericTagFilter(t *testing.T) {

	var empty = map[string]any{}
	filter := NewNumericAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", math.MinInt32, math.MaxInt32, false)

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
			Desc:     "span attribute at the upper limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MaxInt32),
			Decision: Sampled,
		},
		{
			Desc:     "span attribute below min limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MinInt32-1),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute above max limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MaxInt32+1),
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

	var empty = map[string]any{}
	filter := NewNumericAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", math.MinInt32, math.MaxInt32, true)

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
			Decision: Sampled,
		},
		{
			Desc:     "span attribute at the lower limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MinInt32),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute at the upper limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MaxInt32),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute below min limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MinInt32-1),
			Decision: Sampled,
		},
		{
			Desc:     "span attribute above max limit",
			Trace:    newTraceIntAttrs(empty, "example", math.MaxInt32+1),
			Decision: Sampled,
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
