// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"context"
	"net/url"
	"strings"

	"github.com/DataDog/go-sqllexer"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// querySampleTraceContext preserves application_name precedence and falls back to
// the standard sqlcommenter traceparent field before SQL obfuscation removes it.
// No scrape span context is inherited and no vendor-specific keys are required.
func querySampleTraceContext(applicationName, query string) context.Context {
	if ctx := extractQueryTraceparent(applicationName); ctx != nil {
		return ctx
	}
	return extractQueryTraceparent(sqlCommentTraceparent(query))
}

func extractQueryTraceparent(value string) context.Context {
	ctx := (propagation.TraceContext{}).Extract(context.Background(), propagation.MapCarrier{
		traceparentCarrierKey: value,
	})
	if trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return nil
}

func sqlCommentTraceparent(query string) string {
	// Reuse the lexer already used by SQL obfuscation to distinguish comments from
	// quoted literals, identifiers and PostgreSQL dollar-quoted strings.
	lexer := sqllexer.New(query, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	var value string
	found := false
	for {
		token := lexer.Scan()
		switch token.Type {
		case sqllexer.EOF:
			return value
		case sqllexer.ERROR, sqllexer.INCOMPLETE_STRING:
			return ""
		case sqllexer.MULTILINE_COMMENT:
			content := strings.TrimSuffix(strings.TrimPrefix(token.Value, "/*"), "*/")
			// The lexer does not balance nested block comments. Reject the query
			// rather than accidentally interpreting a nested or following token.
			if strings.Contains(content, "/*") {
				return ""
			}
			for pair := range strings.SplitSeq(content, ",") {
				key, encoded, ok := strings.Cut(strings.TrimSpace(pair), "=")
				if !ok {
					continue
				}
				key, err := url.QueryUnescape(strings.TrimSpace(key))
				if err != nil || key != traceparentCarrierKey {
					continue
				}
				// Duplicate keys are ambiguous, including across separate comments.
				if found {
					return ""
				}
				found = true
				encoded = strings.TrimSpace(encoded)
				if len(encoded) < 2 || encoded[0] != '\'' || encoded[len(encoded)-1] != '\'' {
					return ""
				}
				value, err = url.QueryUnescape(encoded[1 : len(encoded)-1])
				if err != nil {
					return ""
				}
			}
		}
	}
}
