// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
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
		Desc          string
		Trace         *TraceData
		ExpectSampled bool
	}{
		{
			Desc:          "non-matching span attribute",
			Trace:         newTraceBoolAttrs(empty, "non_matching", true),
			ExpectSampled: false,
		},
		{
			Desc:          "span attribute with unwanted boolean value",
			Trace:         newTraceBoolAttrs(empty, "example", false),
			ExpectSampled: false,
		},
		{
			Desc:          "span attribute with wanted boolean value",
			Trace:         newTraceBoolAttrs(empty, "example", true),
			ExpectSampled: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			u, _ := uuid.NewRandom()
			TestOTEP235Behavior(t, filter, pcommon.TraceID(u), c.Trace, c.ExpectSampled)
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
		ExpectSampled         bool // Use OTEP 235 boolean expectation instead of exact Decision
		DisableInvertDecision bool
	}{
		{
			Desc:          "invert non-matching span attribute",
			Trace:         newTraceBoolAttrs(empty, "non_matching", true),
			ExpectSampled: true,
		},
		{
			Desc:          "invert span attribute with non matching boolean value",
			Trace:         newTraceBoolAttrs(empty, "example", false),
			ExpectSampled: true,
		},
		{
			Desc:          "invert span attribute with matching boolean value",
			Trace:         newTraceBoolAttrs(empty, "example", true),
			ExpectSampled: false,
		},
		{
			Desc:                  "invert span attribute with non matching boolean value with DisableInvertDecision",
			Trace:                 newTraceBoolAttrs(empty, "example", false),
			ExpectSampled:         true,
			DisableInvertDecision: true,
		},
		{
			Desc:                  "invert span attribute with matching boolean value with DisableInvertDecision",
			Trace:                 newTraceBoolAttrs(empty, "example", true),
			ExpectSampled:         false,
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
			TestOTEP235Behavior(t, filter, pcommon.TraceID(u), c.Trace, c.ExpectSampled)
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
