// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestBooleanTagFilter(t *testing.T) {
	empty := map[string]any{}
	filter := NewBooleanAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", true, false)

	resAttr := map[string]any{}
	resAttr["example"] = 8

	cases := []struct {
		Desc     string
		Trace    *TraceData
		Decision Decision
	}{
		{
			Desc:     "non-matching span attribute",
			Trace:    newTraceBoolAttrs(empty, "non_matching", true),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute with unwanted boolean value",
			Trace:    newTraceBoolAttrs(empty, "example", false),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute with wanted boolean value",
			Trace:    newTraceBoolAttrs(empty, "example", true),
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

func TestBooleanTagFilterInverted(t *testing.T) {
	empty := map[string]any{}
	filter := NewBooleanAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", true, true)

	resAttr := map[string]any{}
	resAttr["example"] = 8

	cases := []struct {
		Desc                  string
		Trace                 *TraceData
		Decision              Decision
		DisableInvertDecision bool
	}{
		{
			Desc:     "non-matching span attribute",
			Trace:    newTraceBoolAttrs(empty, "non_matching", true),
			Decision: InvertSampled,
		},
		{
			Desc:     "span attribute with non matching boolean value",
			Trace:    newTraceBoolAttrs(empty, "example", false),
			Decision: InvertSampled,
		},
		{
			Desc:     "span attribute with matching boolean value",
			Trace:    newTraceBoolAttrs(empty, "example", true),
			Decision: InvertNotSampled,
		},
		{
			Desc:                  "span attribute with non matching boolean value with DisableInvertDecision",
			Trace:                 newTraceBoolAttrs(empty, "example", false),
			Decision:              Sampled,
			DisableInvertDecision: true,
		},
		{
			Desc:                  "span attribute with matching boolean value with DisableInvertDecision",
			Trace:                 newTraceBoolAttrs(empty, "example", true),
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

func newTraceBoolAttrs(nodeAttrs map[string]any, spanAttrKey string, spanAttrValue bool) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	//nolint:errcheck
	rs.Resource().Attributes().FromRaw(nodeAttrs)
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutBool(spanAttrKey, spanAttrValue)
	return &TraceData{
		ReceivedBatches: traces,
	}
}
