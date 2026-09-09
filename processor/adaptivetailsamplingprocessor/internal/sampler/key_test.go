// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	key := ExtractKey(spans, mustSelectors(t, `any.attributes["http.method"]`, `any.attributes["service.name"]`), nil)
	assert.Equal(t, "GET•api", key)
}

func TestExtractKey_MissingField(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes().PutStr("http.method", "POST")

	key := ExtractKey(collect(rs), mustSelectors(t, `any.attributes["http.method"]`, `any.attributes["service.name"]`), nil)
	assert.Equal(t, "POST•<missing>", key)
}

func TestExtractKey_DistinctValuesSorted(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty().Attributes().PutStr("http.method", "POST")
	ss.Spans().AppendEmpty().Attributes().PutStr("http.method", "GET")

	key := ExtractKey(collect(rs), mustSelectors(t, `any.attributes["http.method"]`), nil)
	assert.Equal(t, "GET,POST", key)
}

func TestExtractKey_AllFieldsMissing(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	key := ExtractKey(collect(rs), mustSelectors(t, `any.attributes["missing"]`), nil)
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
		{"resource_only", `resource.attributes["service.name"]`, "api"},
		{"resource_scope_ignores_span_values", `resource.attributes["http.route"]`, "<missing>"},
		{"scope_attributes", `scope.attributes["lib.version"]`, "1.2"},
		{"span_collects_all", `span.attributes["http.route"]`, "/users,/users/id"},
		{"root_only", `root.attributes["http.route"]`, "/users"},
		{"any_unions_scopes", `any.attributes["service.name"]`, "api,span-level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := ExtractKey(collect(rs), mustSelectors(t, tt.selector), isRoot)
			assert.Equal(t, tt.want, key)
		})
	}
}

func TestExtractKey_RootWithoutMatcher(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes().PutStr("k", "v")

	key := ExtractKey(collect(rs), mustSelectors(t, `root.attributes["k"]`), nil)
	assert.Equal(t, "<missing>", key)
}

func TestParseSelector(t *testing.T) {
	tests := []struct {
		in      string
		wantErr string
	}{
		{in: `resource.attributes["service.name"]`},
		{in: `scope.attributes["lib"]`},
		{in: `span.attributes["http.route"]`},
		{in: `root.attributes["http.status_code"]`},
		{in: `any.attributes["k"]`},
		{in: `service.name`, wantErr: "not a scoped attribute selector"},
		{in: `spans.attributes["k"]`, wantErr: "not a scoped attribute selector"},
		{in: `span.attributes[k]`, wantErr: "must have the form"},
		{in: `span.attributes[""]`, wantErr: "single attribute"},
		{in: `span.attributes["a""b"]`, wantErr: "single attribute"},
		{in: `span.name`, wantErr: "must have the form"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := ParseSelector(tt.in)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
