// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func TestRateLimiter(t *testing.T) {
	trace := newTraceStringAttrs(nil, "example", "value")
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rateLimiter := NewRateLimiting(componenttest.NewNopTelemetrySettings(), 3)

	// In Bottom-K approach, rate limiting policies always return AlwaysSample with metadata
	// The actual rate enforcement happens in the bucket manager's second pass
	expectedDecision := Decision{
		Threshold: sampling.AlwaysSampleThreshold,
		Attributes: map[string]any{
			"rate_limiting_policy": true,
			"spans_per_second":     int64(3),
			"policy_type":          "rate_limiting",
		},
	}

	// Test with different span counts - all should return the same decision
	testCases := []struct {
		name      string
		spanCount int64
	}{
		{"Trace span count greater than spans per second", 10},
		{"Trace span count equal to spans per second", 3},
		{"Trace span count less than spans per second", 2},
		{"Trace span count zero", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			traceSpanCount := &atomic.Int64{}
			traceSpanCount.Store(tc.spanCount)
			trace.SpanCount = traceSpanCount

			decision, err := rateLimiter.Evaluate(context.Background(), traceID, trace)
			assert.NoError(t, err)
			assert.Equal(t, expectedDecision, decision)
		})
	}
}
