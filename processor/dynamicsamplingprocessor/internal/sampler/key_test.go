// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	key := ExtractKey(spans, []string{"http.method", "service.name"})
	assert.Equal(t, "GET•api", key)
}

func TestExtractKey_MissingField(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes().PutStr("http.method", "POST")

	key := ExtractKey(collect(rs), []string{"http.method", "service.name"})
	assert.Equal(t, "POST•<missing>", key)
}

func TestExtractKey_DistinctValuesSorted(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty().Attributes().PutStr("http.method", "POST")
	ss.Spans().AppendEmpty().Attributes().PutStr("http.method", "GET")

	key := ExtractKey(collect(rs), []string{"http.method"})
	assert.Equal(t, "GET,POST", key)
}

func TestExtractKey_AllFieldsMissing(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	key := ExtractKey(collect(rs), []string{"missing"})
	assert.Equal(t, "<missing>", key)
}

func collect(rss ...ptrace.ResourceSpans) []ptrace.ResourceSpans {
	return rss
}
