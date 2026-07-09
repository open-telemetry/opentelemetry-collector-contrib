// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestParseCondition_AttributeEquals(t *testing.T) {
	c, err := parseCondition(`http.method == "GET"`)
	require.NoError(t, err)
	got, ok := c.(attributeEqualsCondition)
	require.True(t, ok)
	assert.Equal(t, "http.method", got.field)
	assert.Equal(t, "GET", got.value)
	assert.False(t, got.negated)
}

func TestParseCondition_AttributeNotEquals(t *testing.T) {
	c, err := parseCondition(`service.name != prod`)
	require.NoError(t, err)
	got, ok := c.(attributeEqualsCondition)
	require.True(t, ok)
	assert.True(t, got.negated)
	assert.Equal(t, "prod", got.value)
}

func TestParseCondition_StatusCode(t *testing.T) {
	c, err := parseCondition(`status.code == 2`)
	require.NoError(t, err)
	got, ok := c.(statusCodeCondition)
	require.True(t, ok)
	assert.Equal(t, 2, got.code)
	assert.False(t, got.negated)
}

func TestParseCondition_Errors(t *testing.T) {
	cases := []string{
		"",
		"no operator",
		"=== bad",
		"field == ",
		" == value",
		"status.code == notanumber",
	}
	for _, tc := range cases {
		_, err := parseCondition(tc)
		assert.Error(t, err, "expected error for %q", tc)
	}
}

func TestCondition_AttributeEquals_Matches(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "api")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "GET")

	spans := []ptrace.ResourceSpans{rs}

	tests := []struct {
		expr    string
		matches bool
	}{
		{`http.method == GET`, true},
		{`http.method == POST`, false},
		{`http.method != POST`, true},
		{`service.name == api`, true},
		{`unknown == value`, false},
		{`unknown != value`, true},
	}
	for _, tt := range tests {
		c, err := parseCondition(tt.expr)
		require.NoError(t, err, tt.expr)
		assert.Equal(t, tt.matches, c.matches(spans), tt.expr)
	}
}

func TestCondition_StatusCode_Matches(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.Status().SetCode(ptrace.StatusCodeError)

	spans := []ptrace.ResourceSpans{rs}

	c, err := parseCondition(`status.code == 2`)
	require.NoError(t, err)
	assert.True(t, c.matches(spans))

	c, err = parseCondition(`status.code == 1`)
	require.NoError(t, err)
	assert.False(t, c.matches(spans))

	c, err = parseCondition(`status.code != 2`)
	require.NoError(t, err)
	assert.False(t, c.matches(spans))
}

func TestRule_MatchesAllConditions(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "api")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Status().SetCode(ptrace.StatusCodeError)

	r := &rule{
		name: "errors-from-api",
		conditions: []condition{
			mustCondition(t, `service.name == api`),
			mustCondition(t, `status.code == 2`),
		},
	}
	assert.True(t, r.matches([]ptrace.ResourceSpans{rs}))

	r.conditions[1] = mustCondition(t, `status.code == 1`)
	assert.False(t, r.matches([]ptrace.ResourceSpans{rs}))
}

func TestRule_EmptyConditionsAlwaysMatches(t *testing.T) {
	r := &rule{name: "catchall"}
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	assert.True(t, r.matches([]ptrace.ResourceSpans{rs}))
}

func mustCondition(t *testing.T, expr string) condition {
	t.Helper()
	c, err := parseCondition(expr)
	require.NoError(t, err)
	return c
}
