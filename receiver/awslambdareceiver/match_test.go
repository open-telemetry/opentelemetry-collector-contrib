// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver

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
		// AWS default patterns — literal prefix + single-segment wildcard
		{
			name:      "VPC Flow Logs matches",
			pattern:   "AWSLogs/*/vpcflowlogs",
			objectKey: "AWSLogs/123456789012/vpcflowlogs/us-east-1/2024/01/15/file.log.gz",
			want:      true,
		},
		{
			name:      "CloudTrail matches",
			pattern:   "AWSLogs/*/CloudTrail",
			objectKey: "AWSLogs/123456789012/CloudTrail/us-east-1/2024/01/15/file.json.gz",
			want:      true,
		},
		{
			name:      "ELB Access matches",
			pattern:   "AWSLogs/*/elasticloadbalancing",
			objectKey: "AWSLogs/123456789012/elasticloadbalancing/us-east-1/2024/01/15/file.log.gz",
			want:      true,
		},
		{
			name:      "WAF logs matches",
			pattern:   "AWSLogs/*/WAFLogs",
			objectKey: "AWSLogs/123456789012/WAFLogs/my-web-acl/2024/01/15/file.log.gz",
			want:      true,
		},
		{
			name:      "VPC pattern rejects CloudTrail key",
			pattern:   "AWSLogs/*/vpcflowlogs",
			objectKey: "AWSLogs/123456789012/CloudTrail/us-east-1/file.json.gz",
			want:      false,
		},
		// Specific account override — exact literal at account segment
		{
			name:      "specific account matches correct account",
			pattern:   "AWSLogs/123456789012/vpcflowlogs",
			objectKey: "AWSLogs/123456789012/vpcflowlogs/us-east-1/file.log.gz",
			want:      true,
		},
		{
			name:      "specific account rejects wrong account",
			pattern:   "AWSLogs/123456789012/vpcflowlogs",
			objectKey: "AWSLogs/999999999999/vpcflowlogs/us-east-1/file.log.gz",
			want:      false,
		},
		// Catch-all and edge cases
		{
			name:      "catch-all * matches any path",
			pattern:   "*",
			objectKey: "AWSLogs/123456789012/vpcflowlogs/file.log.gz",
			want:      true,
		},
		{
			name:      "custom path with wildcard matches deeper key",
			pattern:   "my-app/*/logs",
			objectKey: "my-app/production/logs/2024/app.log",
			want:      true,
		},
		{
			name:      "path shorter than pattern does not match",
			pattern:   "AWSLogs/*/vpcflowlogs/us-east-1",
			objectKey: "AWSLogs/123456789012/vpcflowlogs/file.log.gz",
			want:      false,
		},
		// Two consecutive wildcards
		{
			name:      "two wildcards then exact segment matches",
			pattern:   "*/*/WAFLogs",
			objectKey: "AWSLogs/123456789012/WAFLogs/my-web-acl/2024/01/15/file.log.gz",
			want:      true,
		},
		{
			name:      "two wildcards then exact segment rejects mismatch",
			pattern:   "*/*/WAFLogs",
			objectKey: "AWSLogs/123456789012/vpcflowlogs/us-east-1/file.log.gz",
			want:      false,
		},
		// Affix wildcards — prefix
		{
			name:      "prefix wildcard matches eni-*",
			pattern:   "eni-*",
			objectKey: "eni-0abc123def-all",
			want:      true,
		},
		{
			name:      "prefix wildcard rejects wrong prefix",
			pattern:   "eni-*",
			objectKey: "foo-bar",
			want:      false,
		},
		{
			name:      "prefix wildcard matches empty suffix",
			pattern:   "foo*",
			objectKey: "foo",
			want:      true,
		},
		// Affix wildcards — suffix
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
			name:      "suffix wildcard matches empty prefix",
			pattern:   "*foo",
			objectKey: "foo",
			want:      true,
		},
		// Affix wildcards — contains
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
		{
			name:      "contains wildcard matches substring equal to trim",
			pattern:   "*foo*",
			objectKey: "foo",
			want:      true,
		},
		// Mid-segment rejection: foo*bar is not supported — matcher returns false.
		{
			name:      "mid-segment glob rejected (foo*bar)",
			pattern:   "foo*bar",
			objectKey: "foobazbar",
			want:      false,
		},
		// Affix combined with path segments (CW log-group style).
		{
			name:      "affix within a path segment matches",
			pattern:   "/aws/lambda/payment-*",
			objectKey: "/aws/lambda/payment-service/2024/logs",
			want:      true,
		},
		{
			name:      "affix within a path segment rejects mismatch",
			pattern:   "/aws/lambda/payment-*",
			objectKey: "/aws/lambda/other-service/2024/logs",
			want:      false,
		},
		// WAF-style prefix wildcard as log group name.
		{
			name:      "affix waf pattern matches aws-waf-logs-mywebacl",
			pattern:   "aws-waf-logs-*",
			objectKey: "aws-waf-logs-mywebacl",
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
			a:      "AWSLogs/123456789012/vpcflowlogs",
			b:      "AWSLogs/*/vpcflowlogs",
			expect: "a_wins",
		},
		{
			name:   "longer pattern beats shorter",
			a:      "AWSLogs/*/vpcflowlogs/us-east-1",
			b:      "AWSLogs/*/vpcflowlogs",
			expect: "a_wins",
		},
		{
			name:   "identical patterns are equal",
			a:      "AWSLogs/*/vpcflowlogs",
			b:      "AWSLogs/*/vpcflowlogs",
			expect: "equal",
		},
		{
			name:   "wildcard-only beats catch-all (longer wins)",
			a:      "AWSLogs/*/*",
			b:      "*",
			expect: "a_wins",
		},
		// Affix specificity tier (between exact and full wildcard).
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
		{
			name:   "two single-segment affix patterns are equal specificity",
			a:      "eni-*",
			b:      "*-foo",
			expect: "equal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := comparePatternSpecificity(splitForTest(tt.a), splitForTest(tt.b))
			switch tt.expect {
			case "a_wins":
				assert.Negative(t, got, "expected a=%q to be more specific than b=%q", tt.a, tt.b)
			case "b_wins":
				assert.Positive(t, got, "expected b=%q to be more specific than a=%q", tt.b, tt.a)
			case "equal":
				assert.Zero(t, got, "expected a=%q and b=%q to be equal specificity", tt.a, tt.b)
			}
		})
	}
}
