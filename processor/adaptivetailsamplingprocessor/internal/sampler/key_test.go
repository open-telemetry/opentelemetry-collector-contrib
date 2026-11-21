// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestExtractKey_ResourceAndSpanAttributes(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "api")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "GET")

	spans := collect(rs)
	key := ExtractKey(spans, mustSelectors(t, `any.attributes["http.method"]`, `any.attributes["service.name"]`), nil, nil)
	assert.Equal(t, "GET•api", key)
}

func TestExtractKey_MissingField(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes().PutStr("http.method", "POST")

	key := ExtractKey(collect(rs), mustSelectors(t, `any.attributes["http.method"]`, `any.attributes["service.name"]`), nil, nil)
	assert.Equal(t, "POST•<missing>", key)
}

func TestExtractKey_DistinctValuesSorted(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty().Attributes().PutStr("http.method", "POST")
	ss.Spans().AppendEmpty().Attributes().PutStr("http.method", "GET")

	key := ExtractKey(collect(rs), mustSelectors(t, `any.attributes["http.method"]`), nil, nil)
	assert.Equal(t, "GET,POST", key)
}

func TestExtractKey_AllFieldsMissing(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	key := ExtractKey(collect(rs), mustSelectors(t, `any.attributes["missing"]`), nil, nil)
	assert.Equal(t, "<missing>", key)
}

func collect(rss ...ptrace.ResourceSpans) []ptrace.ResourceSpans {
	return rss
}

func mustSelectors(t *testing.T, entries ...string) []Selector {
	t.Helper()
	selectors, err := ParseSelectors(entries)
	require.NoError(t, err)
	return selectors
}

func TestExtractKey_ScopedSelectors(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "api")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().Attributes().PutStr("lib.version", "1.2")
	root := ss.Spans().AppendEmpty()
	root.Attributes().PutStr("http.route", "/users")
	root.Attributes().PutStr("service.name", "span-level") // must NOT leak into resource scope
	child := ss.Spans().AppendEmpty()
	child.Attributes().PutStr("http.route", "/users/id")

	isRoot := func(_ ptrace.ResourceSpans, _ ptrace.ScopeSpans, span ptrace.Span) bool {
		return span == root
	}

	tests := []struct {
		name     string
		selector string
		want     string
	}{
		{"resource_all", `resource.attributes["service.name"]`, "api"},
		{"resource_ignores_span_values", `resource.attributes["http.route"]`, "<missing>"},
		{"scope_all", `scope.attributes["lib.version"]`, "1.2"},
		{"span_collects_all", `span.attributes["http.route"]`, "/users,/users/id"},
		{"root_span", `root.span.attributes["http.route"]`, "/users"},
		{"root_resource_precise", `root.resource.attributes["service.name"]`, "api"},
		{"root_span_precise", `root.span.attributes["service.name"]`, "span-level"},
		{"root_scope", `root.scope.attributes["lib.version"]`, "1.2"},
		{"root_any_unions", `root.any.attributes["service.name"]`, "api,span-level"},
		{"any_unions_scopes", `any.attributes["service.name"]`, "api,span-level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := ExtractKey(collect(rs), mustSelectors(t, tt.selector), isRoot, nil)
			assert.Equal(t, tt.want, key)
		})
	}
}

func TestExtractKey_RootWithoutMatcher(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes().PutStr("k", "v")

	key := ExtractKey(collect(rs), mustSelectors(t, `root.span.attributes["k"]`), nil, nil)
	assert.Equal(t, "<missing>", key)
}

func TestParseSelector(t *testing.T) {
	tests := []struct {
		in      string
		wantErr string
		want    Selector
	}{
		{in: `resource.attributes["service.name"]`},
		{in: `scope.attributes["lib"]`},
		{in: `span.attributes["http.route"]`},
		{in: `any.attributes["k"]`},
		{in: `root.resource.attributes["service.name"]`},
		{in: `root.scope.attributes["lib"]`},
		{in: `root.span.attributes["http.status_code"]`},
		{in: `root.any.attributes["k"]`},
		{in: `span.duration`, want: Selector{Kind: KindSpanDuration, Origin: OriginSpan}},
		{in: `root.span.duration`, want: Selector{Kind: KindSpanDuration, Root: true, Origin: OriginSpan}},
		{in: `trace.duration`, want: Selector{Kind: KindTraceDuration}},
		{in: `root.attributes["http.status_code"]`, wantErr: "root must name an origin"},
		{in: `root.trace.duration`, wantErr: "root.trace.duration is invalid"},
		{in: `resource.duration`, wantErr: "duration selectors must be"},
		{in: `service.name`, wantErr: "not a scoped attribute or duration selector"},
		{in: `spans.attributes["k"]`, wantErr: "not a scoped attribute or duration selector"},
		{in: `span.attributes[k]`, wantErr: "must have the form"},
		{in: `span.attributes[""]`, wantErr: "single attribute"},
		{in: `span.attributes["a""b"]`, wantErr: "single attribute"},
		{in: `span.name`, wantErr: "must have the form"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseSelector(tt.in)
			if tt.wantErr == "" {
				require.NoError(t, err)
				if tt.want != (Selector{}) {
					assert.Equal(t, tt.want, got)
				}
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDurationBucketLabel(t *testing.T) {
	buckets := []time.Duration{2 * time.Millisecond, 24 * time.Millisecond, time.Second}
	assert.Equal(t, "<2ms", DurationBucketLabel(time.Millisecond, buckets))
	assert.Equal(t, "2ms-24ms", DurationBucketLabel(2*time.Millisecond, buckets))
	assert.Equal(t, "2ms-24ms", DurationBucketLabel(10*time.Millisecond, buckets))
	assert.Equal(t, "24ms-1s", DurationBucketLabel(24*time.Millisecond, buckets))
	assert.Equal(t, ">=1s", DurationBucketLabel(time.Second, buckets))
	assert.Equal(t, ">=1s", DurationBucketLabel(2*time.Second, buckets))
}

func TestExtractKey_SpanAndTraceDuration(t *testing.T) {
	buckets := []time.Duration{10 * time.Millisecond, 100 * time.Millisecond}
	// StartTimestamp 0 is treated as unset, so avoid the unix epoch.
	base := time.Unix(1_700_000_000, 0).UTC()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	fast := ss.Spans().AppendEmpty()
	fast.SetStartTimestamp(pcommon.NewTimestampFromTime(base))
	fast.SetEndTimestamp(pcommon.NewTimestampFromTime(base.Add(5 * time.Millisecond)))
	slow := ss.Spans().AppendEmpty()
	slow.SetStartTimestamp(pcommon.NewTimestampFromTime(base))
	slow.SetEndTimestamp(pcommon.NewTimestampFromTime(base.Add(50 * time.Millisecond)))

	key := ExtractKey(collect(rs), mustSelectors(t, "span.duration"), nil, buckets)
	assert.Equal(t, "10ms-100ms,<10ms", key)

	key = ExtractKey(collect(rs), mustSelectors(t, "trace.duration"), nil, buckets)
	assert.Equal(t, "10ms-100ms", key)

	key = ExtractKey(collect(rs), mustSelectors(t, "root.span.duration"), func(_ ptrace.ResourceSpans, _ ptrace.ScopeSpans, span ptrace.Span) bool {
		return span == slow
	}, buckets)
	assert.Equal(t, "10ms-100ms", key)

	key = ExtractKey(collect(rs), mustSelectors(t, "root.span.duration"), nil, buckets)
	assert.Equal(t, "<missing>", key)
}

func TestValidateDurationBuckets(t *testing.T) {
	require.NoError(t, ValidateDurationBuckets([]time.Duration{time.Millisecond, time.Second}))
	require.ErrorContains(t, ValidateDurationBuckets(nil), "at least one")
	require.ErrorContains(t, ValidateDurationBuckets([]time.Duration{0}), "greater than zero")
	require.ErrorContains(t, ValidateDurationBuckets([]time.Duration{time.Second, time.Millisecond}), "strictly increasing")
}
