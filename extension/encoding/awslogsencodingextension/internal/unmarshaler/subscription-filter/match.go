// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"

import "strings"

// catchAllPattern is the bare wildcard that matches any single-segment path.
// Use this as a fallback/catch-all entry.
const catchAllPattern = "*"

// matchPrefixWithWildcard reports whether targetParts has patternParts as a
// prefix. Each pattern segment may be:
//   - an exact literal — compared by string equality
//   - "*" — matches any single segment
//   - an affix wildcard within a single segment:
//     "foo*" / "*foo" / "*foo*"
//
// Mid-segment globs such as "foo*bar" are not supported and never match.
// Callers split paths on "/" before invoking. For non-path strings such as a
// CloudWatch log_stream name, callers pass a single-element slice.
// The target may contain extra trailing segments beyond the pattern's length.
func matchPrefixWithWildcard(targetParts, patternParts []string) bool {
	// Target must have at least as many segments as the pattern.
	if len(targetParts) < len(patternParts) {
		return false
	}

	for i, p := range patternParts {
		if p == catchAllPattern {
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
// Supported pattern shapes:
//   - "foo*"   — target must start with "foo"
//   - "*foo"   — target must end with "foo"
//   - "*foo*"  — target must contain "foo"
//
// Any other use of "*" inside the pattern (mid-segment, more than two
// occurrences) returns false.
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
		return false
	}
}

// comparePatternSpecificity orders two pre-split patterns by specificity.
// It returns a negative value if a is more specific than b, positive if b
// is more specific, and zero if they are equally specific.
//
// Rules:
//   - At each position, an exact segment is more specific than an affix
//     wildcard, which is more specific than the bare "*" wildcard.
//   - When all shared positions are equally specific, the longer pattern wins.
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

// segmentSpecificity returns a score for a single pattern segment.
// Higher score = more specific. The score is:
//   - 2 for an exact literal (no '*')
//   - 1 for an affix wildcard (contains '*' but isn't bare "*")
//   - 0 for the bare "*"
func segmentSpecificity(segment string) int {
	if segment == catchAllPattern {
		return 0
	}
	if strings.ContainsRune(segment, '*') {
		return 1
	}
	return 2
}
