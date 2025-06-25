// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// testOTEP235BehaviorStatusCode tests sampling decision using proper OTEP 235 threshold logic
// for status code filter tests. Uses fixed randomness values to make tests deterministic.
func testOTEP235BehaviorStatusCode(t *testing.T, filter PolicyEvaluator, traceID pcommon.TraceID, trace *TraceData, expectSampled bool) {
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

func TestNewStatusCodeFilter_errorHandling(t *testing.T) {
	_, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{})
	assert.Error(t, err, "expected at least one status code to filter on")

	_, err = NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"OK", "ERR"})
	assert.EqualError(t, err, "unknown status code \"ERR\", supported: OK, ERROR, UNSET")
}

func TestStatusCodeSampling(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc                  string
		StatusCodesToFilterOn []string
		StatusCodesPresent    []ptrace.StatusCode
		ExpectSample          bool
	}{
		{
			Desc:                  "filter on ERROR - none match",
			StatusCodesToFilterOn: []string{"ERROR"},
			StatusCodesPresent:    []ptrace.StatusCode{ptrace.StatusCodeOk, ptrace.StatusCodeUnset, ptrace.StatusCodeOk},
			ExpectSample:          false,
		},
		{
			Desc:                  "filter on OK and ERROR - none match",
			StatusCodesToFilterOn: []string{"OK", "ERROR"},
			StatusCodesPresent:    []ptrace.StatusCode{ptrace.StatusCodeUnset, ptrace.StatusCodeUnset},
			ExpectSample:          false,
		},
		{
			Desc:                  "filter on UNSET - matches",
			StatusCodesToFilterOn: []string{"UNSET"},
			StatusCodesPresent:    []ptrace.StatusCode{ptrace.StatusCodeUnset},
			ExpectSample:          true,
		},
		{
			Desc:                  "filter on OK and UNSET - matches",
			StatusCodesToFilterOn: []string{"OK", "UNSET"},
			StatusCodesPresent:    []ptrace.StatusCode{ptrace.StatusCodeError, ptrace.StatusCodeOk},
			ExpectSample:          true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ils := rs.ScopeSpans().AppendEmpty()

			for _, statusCode := range c.StatusCodesPresent {
				span := ils.Spans().AppendEmpty()
				span.Status().SetCode(statusCode)
				span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
				span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
			}

			trace := &TraceData{
				ReceivedBatches: traces,
			}

			statusCodeFilter, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), c.StatusCodesToFilterOn)
			assert.NoError(t, err)

			testOTEP235BehaviorStatusCode(t, statusCodeFilter, traceID, trace, c.ExpectSample)
		})
	}
}
