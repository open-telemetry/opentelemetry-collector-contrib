// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import "strings"

// catchAllPattern is a special pattern value that matches any path.
// Use this as a fallback/catch-all encoding entry.
const catchAllPattern = "*"

// matchPrefixWithWildcard checks whether targetParts has patternParts as a prefix.
// Each pattern segment may be one of:
//   - an exact literal (compared for string equality)
//   - "*" — matches any single segment
//   - a prefix wildcard "foo*" — segment must start with "foo"
//   - a suffix wildcard "*foo" — segment must end with "foo"
//   - a contains wildcard "*foo*" — segment must contain "foo"
//
// Mid-segment wildcards such as "foo*bar" are not supported and always return false.
// The target may have more segments than the pattern (prefix semantics).
//
// The caller is responsible for splitting the object key and the pattern on "/" before calling.
//
// Examples:
//
//	matchPrefixWithWildcard(
//	    []string{"AWSLogs", "123456789012", "vpcflowlogs", "file.gz"},
//	    []string{"AWSLogs", "*", "vpcflowlogs"},
//	) => true
//	matchPrefixWithWildcard([]string{"any", "path"}, []string{"*"}) => true
//	matchPrefixWithWildcard([]string{"eni-0abc123-all"}, []string{"eni-*"}) => true
//	matchPrefixWithWildcard([]string{"123_CloudTrail_us-east-1"}, []string{"*_CloudTrail_*"}) => true
func matchPrefixWithWildcard(targetParts, patternParts []string) bool {
	// Target must have at least as many segments as the pattern.
	if len(targetParts) < len(patternParts) {
		return false
	}

	for i, p := range patternParts {
		if p == "*" {
			continue // wildcard matches any single segment
		}
		if strings.ContainsRune(p, '*') {
			if !matchAffix(targetParts[i], p) {
				return false
			}
			continue
		}
		if p != targetParts[i] {
			return false
		}
	}

	return true
}

// matchAffix matches a single segment against an affix-wildcard pattern.
// Supported forms:
//   - "foo*"   — target must start with "foo"
//   - "*foo"   — target must end with "foo"
//   - "*foo*"  — target must contain "foo"
//
// Unsupported forms (return false):
//   - "foo*bar" — mid-segment glob
//   - patterns with more than two '*' characters
//   - patterns with a single '*' that is not at the start or end of the segment
func matchAffix(target, pattern string) bool {
	starCount := strings.Count(pattern, "*")
	hasPrefix := strings.HasPrefix(pattern, "*")
	hasSuffix := strings.HasSuffix(pattern, "*")
	switch {
	case starCount == 1 && hasSuffix:
		return strings.HasPrefix(target, strings.TrimSuffix(pattern, "*"))
	case starCount == 1 && hasPrefix:
		return strings.HasSuffix(target, strings.TrimPrefix(pattern, "*"))
	case starCount == 2 && hasPrefix && hasSuffix:
		return strings.Contains(target, strings.Trim(pattern, "*"))
	default:
		return false // unsupported: foo*bar, **foo, etc.
	}
}

// comparePatternSpecificity compares two pre-split patterns by specificity.
// Returns negative if a is more specific, positive if b is more specific, 0 if equal.
//
// Rules (evaluated left-to-right per segment):
//   - Exact segment beats affix wildcard beats full wildcard at the same position.
//   - Longer pattern (more segments) beats a shorter one.
func comparePatternSpecificity(partsA, partsB []string) int {
	minLen := min(len(partsA), len(partsB))

	for i := range minLen {
		aSpec := segmentSpecificity(partsA[i])
		bSpec := segmentSpecificity(partsB[i])
		if aSpec != bSpec {
			// Higher score = more specific = should sort earlier.
			return bSpec - aSpec
		}
	}

	// Longer pattern is more specific.
	return len(partsB) - len(partsA)
}

// segmentSpecificity returns a specificity score for a single pattern segment.
// Higher score = more specific.
//   - 2: exact literal (no '*')
//   - 1: affix wildcard (contains '*' but isn't bare "*")
//   - 0: full wildcard "*"
func segmentSpecificity(segment string) int {
	if segment == "*" {
		return 0
	}
	if strings.ContainsRune(segment, '*') {
		return 1
	}
	return 2
}

// isCatchAllPattern returns true if the pattern is the bare catch-all "*".
func isCatchAllPattern(pattern string) bool {
	return pattern == catchAllPattern
}
