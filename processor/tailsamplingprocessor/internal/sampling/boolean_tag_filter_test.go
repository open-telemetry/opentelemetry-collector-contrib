// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestBooleanTagFilter(t *testing.T) {

	var empty = map[string]interface{}{}
	filter := NewBooleanAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", true)

	resAttr := map[string]interface{}{}
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

func newTraceBoolAttrs(nodeAttrs map[string]interface{}, spanAttrKey string, spanAttrValue bool) *TraceData {
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
