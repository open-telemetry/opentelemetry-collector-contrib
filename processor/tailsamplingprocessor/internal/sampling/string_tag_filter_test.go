// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// TestStringAttributeCfg is replicated with StringAttributeCfg
type TestStringAttributeCfg struct {
	Key                  string
	Values               []string
	EnabledRegexMatching bool
	CacheMaxSize         int
	InvertMatch          bool
}

// testOTEP235Behavior tests sampling decision using proper OTEP 235 threshold logic
// instead of boolean comparison. Uses fixed randomness values to make tests deterministic.
func testOTEP235Behavior(t *testing.T, filter PolicyEvaluator, traceID pcommon.TraceID, trace *TraceData, expectSampled bool) {
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

func TestStringTagFilter(t *testing.T) {
	cases := []struct {
		Desc                  string
		Trace                 *TraceData
		filterCfg             *TestStringAttributeCfg
		ExpectSampled         bool // Use OTEP 235 boolean expectation instead of exact Decision
		DisableInvertDecision bool
	}{
		{
			Desc:          "nonmatching node attribute key",
			Trace:         newTraceStringAttrs(map[string]any{"non_matching": "value"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			ExpectSampled: false,
		},
		{
			Desc:          "nonmatching node attribute value",
			Trace:         newTraceStringAttrs(map[string]any{"example": "non_matching"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			ExpectSampled: false,
		},
		{
			Desc:          "matching node attribute",
			Trace:         newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			ExpectSampled: true,
		},
		{
			Desc:          "nonmatching span attribute key",
			Trace:         newTraceStringAttrs(nil, "nonmatching", "value"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			ExpectSampled: false,
		},
		{
			Desc:          "nonmatching span attribute value",
			Trace:         newTraceStringAttrs(nil, "example", "nonmatching"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			ExpectSampled: false,
		},
		{
			Desc:          "matching span attribute",
			Trace:         newTraceStringAttrs(nil, "example", "value"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			ExpectSampled: true,
		},
		{
			Desc:          "matching span attribute with regex",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"v[0-9]+.HealthCheck$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize},
			ExpectSampled: true,
		},
		{
			Desc:          "nonmatching span attribute with regex",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"v[a-z]+.HealthCheck$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize},
			ExpectSampled: false,
		},
		{
			Desc:          "matching span attribute with regex without CacheSize provided in config",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"v[0-9]+.HealthCheck$"}, EnabledRegexMatching: true},
			ExpectSampled: true,
		},
		{
			Desc:          "matching plain text node attribute in regex",
			Trace:         newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize},
			ExpectSampled: true,
		},
		{
			Desc:          "nonmatching span attribute on empty filter list",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{}, EnabledRegexMatching: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert nonmatching node attribute key",
			Trace:         newTraceStringAttrs(map[string]any{"non_matching": "value"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:          "invert nonmatching node attribute value",
			Trace:         newTraceStringAttrs(map[string]any{"example": "non_matching"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:          "invert nonmatching node attribute list",
			Trace:         newTraceStringAttrs(map[string]any{"example": "non_matching"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:          "invert matching node attribute",
			Trace:         newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert matching node attribute list",
			Trace:         newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert nonmatching span attribute key",
			Trace:         newTraceStringAttrs(nil, "nonmatching", "value"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:          "invert nonmatching span attribute value",
			Trace:         newTraceStringAttrs(nil, "example", "nonmatching"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:          "invert nonmatching span attribute list",
			Trace:         newTraceStringAttrs(nil, "example", "nonmatching"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:          "invert matching span attribute",
			Trace:         newTraceStringAttrs(nil, "example", "value"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert matching span attribute list",
			Trace:         newTraceStringAttrs(nil, "example", "value"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert matching span attribute with regex",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"v[0-9]+.HealthCheck$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert matching span attribute with regex list",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"^http", "v[0-9]+.HealthCheck$", "metrics$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert nonmatching span attribute with regex",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"v[a-z]+.HealthCheck$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:          "invert nonmatching span attribute with regex list",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"^http", "v[a-z]+.HealthCheck$", "metrics$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:          "invert matching plain text node attribute in regex",
			Trace:         newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert matching plain text node attribute in regex list",
			Trace:         newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled: false,
		},
		{
			Desc:          "invert nonmatching span attribute on empty filter list",
			Trace:         newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg:     &TestStringAttributeCfg{Key: "example", Values: []string{}, EnabledRegexMatching: true, InvertMatch: true},
			ExpectSampled: true,
		},
		{
			Desc:                  "invert matching node attribute key with DisableInvertDecision",
			Trace:                 newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg:             &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled:         false,
			DisableInvertDecision: true,
		},
		{
			Desc:                  "invert nonmatching node attribute key with DisableInvertDecision",
			Trace:                 newTraceStringAttrs(map[string]any{"non_matching": "value"}, "", ""),
			filterCfg:             &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			ExpectSampled:         true,
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
			filter := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), c.filterCfg.Key, c.filterCfg.Values, c.filterCfg.EnabledRegexMatching, c.filterCfg.CacheMaxSize, c.filterCfg.InvertMatch)

			// Use new OTEP 235 threshold-based testing
			traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
			testOTEP235Behavior(t, filter, traceID, c.Trace, c.ExpectSampled)
		})
	}
}

func BenchmarkStringTagFilterEvaluatePlainText(b *testing.B) {
	trace := newTraceStringAttrs(map[string]any{"example": "value"}, "", "")
	filter := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", []string{"value"}, false, 0, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := filter.Evaluate(context.Background(), pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}), trace)
		assert.NoError(b, err)
	}
}

func BenchmarkStringTagFilterEvaluateRegex(b *testing.B) {
	trace := newTraceStringAttrs(map[string]any{"example": "grpc.health.v1.HealthCheck"}, "", "")
	filter := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "example", []string{"v[0-9]+.HealthCheck$"}, true, 0, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := filter.Evaluate(context.Background(), pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}), trace)
		assert.NoError(b, err)
	}
}

func newTraceStringAttrs(nodeAttrs map[string]any, spanAttrKey string, spanAttrValue string) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	//nolint:errcheck
	rs.Resource().Attributes().FromRaw(nodeAttrs)
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutStr(spanAttrKey, spanAttrValue)
	return &TraceData{
		ReceivedBatches: traces,
	}
}
