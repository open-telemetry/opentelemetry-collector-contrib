// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// splitForTest splits a "/" separated path string into parts for use in match tests.
func splitForTest(s string) []string {
	return strings.Split(s, "/")
}

func TestMatchPrefixWithWildcard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pattern   string
		objectKey string
		want      bool
	}{
		// Path-style patterns with single-segment wildcard.
		{
			name:      "log group with wildcard segment matches",
			pattern:   "/aws/lambda/*",
			objectKey: "/aws/lambda/my-service",
			want:      true,
		},
		{
			name:      "log group rejects mismatched literal",
			pattern:   "/aws/lambda/*",
			objectKey: "/aws/rds/instance",
			want:      false,
		},
		{
			name:      "deeper key still matches prefix pattern",
			pattern:   "/aws/lambda/*",
			objectKey: "/aws/lambda/my-service/extra/segments",
			want:      true,
		},
		// Exact literal segments.
		{
			name:      "exact literal at any position matches",
			pattern:   "/aws/lambda/payment",
			objectKey: "/aws/lambda/payment",
			want:      true,
		},
		{
			name:      "exact literal rejects different value",
			pattern:   "/aws/lambda/payment",
			objectKey: "/aws/lambda/orders",
			want:      false,
		},
		// Catch-all and length checks.
		{
			name:      "catch-all * matches any path",
			pattern:   "*",
			objectKey: "/anything/here",
			want:      true,
		},
		{
			name:      "path shorter than pattern does not match",
			pattern:   "/aws/lambda/*/extra",
			objectKey: "/aws/lambda/my-service",
			want:      false,
		},
		// Two consecutive wildcards.
		{
			name:      "two wildcards then exact segment matches",
			pattern:   "*/*/payment",
			objectKey: "aws/lambda/payment",
			want:      true,
		},
		{
			name:      "two wildcards then exact segment rejects mismatch",
			pattern:   "*/*/payment",
			objectKey: "aws/lambda/orders",
			want:      false,
		},
		// Affix wildcards — prefix, suffix, contains.
		{
			name:      "prefix wildcard matches eni-*",
			pattern:   "eni-*",
			objectKey: "eni-0abc123def",
			want:      true,
		},
		{
			name:      "prefix wildcard rejects wrong prefix",
			pattern:   "eni-*",
			objectKey: "foo-bar",
			want:      false,
		},
		{
			name:      "suffix wildcard matches *Logs",
			pattern:   "*Logs",
			objectKey: "WAFLogs",
			want:      true,
		},
		{
			name:      "suffix wildcard rejects wrong suffix",
			pattern:   "*Logs",
			objectKey: "vpcflow",
			want:      false,
		},
		{
			name:      "contains wildcard matches *_CloudTrail_*",
			pattern:   "*_CloudTrail_*",
			objectKey: "123456789012_CloudTrail_us-east-1",
			want:      true,
		},
		{
			name:      "contains wildcard rejects when substring missing",
			pattern:   "*_CloudTrail_*",
			objectKey: "vpcflow-logs",
			want:      false,
		},
		// Edge: trim of "*foo*" leaves "foo"; "foo" target should match.
		{
			name:      "contains wildcard matches when target equals the trim",
			pattern:   "*foo*",
			objectKey: "foo",
			want:      true,
		},
		// Mid-segment globs are not supported.
		{
			name:      "mid-segment glob rejected (foo*bar)",
			pattern:   "foo*bar",
			objectKey: "foobazbar",
			want:      false,
		},
		// Affix combined with path segments.
		{
			name:      "affix within a path segment matches",
			pattern:   "/aws/lambda/payment-*",
			objectKey: "/aws/lambda/payment-service/2024/logs",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchPrefixWithWildcard(splitForTest(tt.objectKey), splitForTest(tt.pattern))
			assert.Equal(t, tt.want, got, "matchPrefixWithWildcard(%q, %q)", tt.objectKey, tt.pattern)
		})
	}
}

func TestComparePatternSpecificity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		a, b   string
		expect string // "a_wins" | "b_wins" | "equal"
	}{
		{
			name:   "exact segment beats wildcard at same position",
			a:      "/aws/lambda/payment",
			b:      "/aws/lambda/*",
			expect: "a_wins",
		},
		{
			name:   "longer pattern beats shorter",
			a:      "/aws/lambda/*/sub",
			b:      "/aws/lambda/*",
			expect: "a_wins",
		},
		{
			name:   "identical patterns are equal",
			a:      "/aws/lambda/*",
			b:      "/aws/lambda/*",
			expect: "equal",
		},
		{
			name:   "affix beats full wildcard",
			a:      "eni-*",
			b:      "*",
			expect: "a_wins",
		},
		{
			name:   "exact beats affix at same position",
			a:      "eni-foo",
			b:      "eni-*",
			expect: "a_wins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := comparePatternSpecificity(splitForTest(tt.a), splitForTest(tt.b))
			switch tt.expect {
			case "a_wins":
				assert.Negative(t, got, "expected a=%q more specific than b=%q", tt.a, tt.b)
			case "b_wins":
				assert.Positive(t, got, "expected b=%q more specific than a=%q", tt.b, tt.a)
			case "equal":
				assert.Zero(t, got, "expected a=%q and b=%q equal specificity", tt.a, tt.b)
			}
		})
	}
}

func TestSegmentSpecificity(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 2, segmentSpecificity("payment"))
	assert.Equal(t, 1, segmentSpecificity("payment-*"))
	assert.Equal(t, 0, segmentSpecificity("*"))
}
