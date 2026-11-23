// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestQuerySampleTraceContext(t *testing.T) {
	const parent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	const comment = "/*traceparent='" + parent + "'*/"
	for _, tc := range []struct {
		name, application, query string
		valid                    bool
	}{
		{name: "application name wins", application: parent, query: "/*traceparent='invalid'*/ SELECT 1", valid: true},
		{name: "leading comment", application: "orders", query: comment + " SELECT 1", valid: true},
		{name: "trailing comment", query: "SELECT 1 " + comment, valid: true},
		{name: "several fields", query: "/*db='orders',traceparent='" + parent + "',service='web'*/ SELECT 1", valid: true},
		{name: "whitespace", query: "/*\n traceparent = '" + parent + "' \n*/ SELECT 1", valid: true},
		{name: "url encoded", query: "/*trace%70arent='00%2D4bf92f3577b34da6a3ce929d0e0e4736%2D00f067aa0ba902b7%2D01'*/ SELECT 1", valid: true},
		{name: "literal", query: "SELECT '/*traceparent=''" + parent + "''*/'"},
		{name: "identifier", query: "SELECT \"" + comment + "\" FROM tbl"},
		{name: "dollar quote", query: "SELECT $body$" + comment + "$body$"},
		{name: "line comment", query: "SELECT 1 -- " + comment},
		{name: "nested comment", query: "/* ignored /*other='x'*/ " + comment + " */ SELECT 1"},
		{name: "truncated comment", query: "/*traceparent='" + parent + "'"},
		{name: "leading comment, truncated literal", query: comment + " SELECT * FROM orders WHERE note = 'unfinish", valid: true},
		{name: "leading comment, truncated dollar quote", query: comment + " SELECT $body$unfinish", valid: true},
		{name: "truncated literal before comment", query: "SELECT 'unfinish " + comment},
		{name: "duplicate key", query: "/*traceparent='" + parent + "',traceparent='" + parent + "'*/ SELECT 1"},
		{name: "duplicate comments", query: comment + " SELECT 1 " + comment},
		{name: "missing quotes", query: "/*traceparent=" + parent + "*/ SELECT 1"},
		{name: "invalid encoding", query: "/*traceparent='%zz'*/ SELECT 1"},
		{name: "zero trace ID", query: "/*traceparent='00-00000000000000000000000000000000-00f067aa0ba902b7-01'*/ SELECT 1"},
		{name: "zero span ID", query: "/*traceparent='00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01'*/ SELECT 1"},
		{name: "prefixed application name", application: "app-" + parent, query: "SELECT 1"},
		{name: "absent", application: "orders", query: "SELECT 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := querySampleTraceContext(tc.application, tc.query)
			if !tc.valid {
				require.Nil(t, ctx)
				return
			}
			require.NotNil(t, ctx)
			span := trace.SpanContextFromContext(ctx)
			require.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", span.TraceID().String())
			require.Equal(t, "00f067aa0ba902b7", span.SpanID().String())
			require.True(t, span.IsSampled())
		})
	}
}
