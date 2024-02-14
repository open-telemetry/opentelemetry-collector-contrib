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

// TestStringAttributeCfg is replicated with StringAttributeCfg
type TestStringAttributeCfg struct {
	Key                  string
	Values               []string
	EnabledRegexMatching bool
	CacheMaxSize         int
	InvertMatch          bool
}

func TestStringTagFilter(t *testing.T) {

	cases := []struct {
		Desc      string
		Trace     *TraceData
		filterCfg *TestStringAttributeCfg
		Decision  Decision
	}{
		{
			Desc:      "nonmatching node attribute key",
			Trace:     newTraceStringAttrs(map[string]any{"non_matching": "value"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			Decision:  NotSampled,
		},
		{
			Desc:      "nonmatching node attribute value",
			Trace:     newTraceStringAttrs(map[string]any{"example": "non_matching"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			Decision:  NotSampled,
		},
		{
			Desc:      "matching node attribute",
			Trace:     newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			Decision:  Sampled,
		},
		{
			Desc:      "nonmatching span attribute key",
			Trace:     newTraceStringAttrs(nil, "nonmatching", "value"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			Decision:  NotSampled,
		},
		{
			Desc:      "nonmatching span attribute value",
			Trace:     newTraceStringAttrs(nil, "example", "nonmatching"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			Decision:  NotSampled,
		},
		{
			Desc:      "matching span attribute",
			Trace:     newTraceStringAttrs(nil, "example", "value"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize},
			Decision:  Sampled,
		},
		{
			Desc:      "matching span attribute with regex",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"v[0-9]+.HealthCheck$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize},
			Decision:  Sampled,
		},
		{
			Desc:      "nonmatching span attribute with regex",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"v[a-z]+.HealthCheck$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize},
			Decision:  NotSampled,
		},
		{
			Desc:      "matching span attribute with regex without CacheSize provided in config",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"v[0-9]+.HealthCheck$"}, EnabledRegexMatching: true},
			Decision:  Sampled,
		},
		{
			Desc:      "matching plain text node attribute in regex",
			Trace:     newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize},
			Decision:  Sampled,
		},
		{
			Desc:      "nonmatching span attribute on empty filter list",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{}, EnabledRegexMatching: true},
			Decision:  NotSampled,
		},
		{
			Desc:      "invert nonmatching node attribute key",
			Trace:     newTraceStringAttrs(map[string]any{"non_matching": "value"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertSampled,
		},
		{
			Desc:      "invert nonmatching node attribute value",
			Trace:     newTraceStringAttrs(map[string]any{"example": "non_matching"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertSampled,
		},
		{
			Desc:      "invert nonmatching node attribute list",
			Trace:     newTraceStringAttrs(map[string]any{"example": "non_matching"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertSampled,
		},
		{
			Desc:      "invert matching node attribute",
			Trace:     newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertNotSampled,
		},
		{
			Desc:      "invert matching node attribute list",
			Trace:     newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertNotSampled,
		},
		{
			Desc:      "invert nonmatching span attribute key",
			Trace:     newTraceStringAttrs(nil, "nonmatching", "value"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertSampled,
		},
		{
			Desc:      "invert nonmatching span attribute value",
			Trace:     newTraceStringAttrs(nil, "example", "nonmatching"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertSampled,
		},
		{
			Desc:      "invert nonmatching span attribute list",
			Trace:     newTraceStringAttrs(nil, "example", "nonmatching"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertSampled,
		},
		{
			Desc:      "invert matching span attribute",
			Trace:     newTraceStringAttrs(nil, "example", "value"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertNotSampled,
		},
		{
			Desc:      "invert matching span attribute list",
			Trace:     newTraceStringAttrs(nil, "example", "value"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: false, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertNotSampled,
		},
		{
			Desc:      "invert matching span attribute with regex",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"v[0-9]+.HealthCheck$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertNotSampled,
		},
		{
			Desc:      "invert matching span attribute with regex list",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"^http", "v[0-9]+.HealthCheck$", "metrics$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertNotSampled,
		},
		{
			Desc:      "invert nonmatching span attribute with regex",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"v[a-z]+.HealthCheck$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertSampled,
		},
		{
			Desc:      "invert nonmatching span attribute with regex list",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"^http", "v[a-z]+.HealthCheck$", "metrics$"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertSampled,
		},
		{
			Desc:      "invert matching plain text node attribute in regex",
			Trace:     newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"value"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertNotSampled,
		},
		{
			Desc:      "invert matching plain text node attribute in regex list",
			Trace:     newTraceStringAttrs(map[string]any{"example": "value"}, "", ""),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{"first_value", "value", "last_value"}, EnabledRegexMatching: true, CacheMaxSize: defaultCacheSize, InvertMatch: true},
			Decision:  InvertNotSampled,
		},
		{
			Desc:      "invert nonmatching span attribute on empty filter list",
			Trace:     newTraceStringAttrs(nil, "example", "grpc.health.v1.HealthCheck"),
			filterCfg: &TestStringAttributeCfg{Key: "example", Values: []string{}, EnabledRegexMatching: true, InvertMatch: true},
			Decision:  InvertSampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			filter := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), c.filterCfg.Key, c.filterCfg.Values, c.filterCfg.EnabledRegexMatching, c.filterCfg.CacheMaxSize, c.filterCfg.InvertMatch)
			decision, err := filter.Evaluate(context.Background(), pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}), c.Trace)
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
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
