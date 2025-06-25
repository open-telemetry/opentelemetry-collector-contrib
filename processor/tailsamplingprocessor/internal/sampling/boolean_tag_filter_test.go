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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// testOTEP235BehaviorBoolean tests sampling decision using proper OTEP 235 threshold logic
// for boolean filter tests. Uses fixed randomness values to make tests deterministic.
func testOTEP235BehaviorBoolean(t *testing.T, filter PolicyEvaluator, traceID pcommon.TraceID, trace *TraceData, expectSampled bool) {
	decision, err := filter.Evaluate(context.Background(), traceID, trace)
	assert.NoError(t, err)

	// Test with randomness = 0 (always samples if threshold is AlwaysSampleThreshold)
	randomnessZero, err := sampling.UnsignedToRandomness(0)
	assert.NoError(t, err)

	// Test with randomness near max (only samples if threshold is very high)
	randomnessHigh, err := sampling.UnsignedToRandomness(sampling.MaxAdjustedCount - 1)
	assert.NoError(t, err)

	if expectSampled {
		// If we expect sampling, the decision should have a low threshold that allows sampling
		assert.True(t, decision.ShouldSample(randomnessZero), "Decision should sample with randomness=0")
		// For true "always sample" decisions, even high randomness should work
		if decision.Threshold == sampling.AlwaysSampleThreshold {
			assert.True(t, decision.ShouldSample(randomnessHigh), "AlwaysSampleThreshold should sample with any randomness")
		}
	} else {
		// If we expect no sampling, the decision should have high threshold (NeverSampleThreshold)
		assert.False(t, decision.ShouldSample(randomnessZero), "Decision should not sample with randomness=0")
		assert.False(t, decision.ShouldSample(randomnessHigh), "Decision should not sample with randomness=high")
		assert.Equal(t, sampling.NeverSampleThreshold, decision.Threshold, "Non-sampling decision should have NeverSampleThreshold")
	}
}

func TestBooleanTagFilter(t *testing.T) {
	empty := map[string]any{}
	filter := NewBooleanAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", true, false)

	resAttr := map[string]any{}
	resAttr["example"] = 8

	cases := []struct {
		Desc          string
		Trace         *TraceData
		ExpectSampled bool // Use OTEP 235 boolean expectation instead of exact Decision
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
			testOTEP235BehaviorBoolean(t, filter, pcommon.TraceID(u), c.Trace, c.ExpectSampled)
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
			testOTEP235BehaviorBoolean(t, filter, pcommon.TraceID(u), c.Trace, c.ExpectSampled)
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
