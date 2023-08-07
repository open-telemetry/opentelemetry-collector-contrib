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
)

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
		Decision              Decision
	}{
		{
			Desc:                  "filter on ERROR - none match",
			StatusCodesToFilterOn: []string{"ERROR"},
			StatusCodesPresent:    []ptrace.StatusCode{ptrace.StatusCodeOk, ptrace.StatusCodeUnset, ptrace.StatusCodeOk},
			Decision:              NotSampled,
		},
		{
			Desc:                  "filter on OK and ERROR - none match",
			StatusCodesToFilterOn: []string{"OK", "ERROR"},
			StatusCodesPresent:    []ptrace.StatusCode{ptrace.StatusCodeUnset, ptrace.StatusCodeUnset},
			Decision:              NotSampled,
		},
		{
			Desc:                  "filter on UNSET - matches",
			StatusCodesToFilterOn: []string{"UNSET"},
			StatusCodesPresent:    []ptrace.StatusCode{ptrace.StatusCodeUnset},
			Decision:              Sampled,
		},
		{
			Desc:                  "filter on OK and UNSET - matches",
			StatusCodesToFilterOn: []string{"OK", "UNSET"},
			StatusCodesPresent:    []ptrace.StatusCode{ptrace.StatusCodeError, ptrace.StatusCodeOk},
			Decision:              Sampled,
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

			decision, err := statusCodeFilter.Evaluate(context.Background(), traceID, trace)
			assert.NoError(t, err)
			assert.Equal(t, c.Decision, decision)
		})
	}
}
