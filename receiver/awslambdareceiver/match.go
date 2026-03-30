// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"path"
	"strings"
)

// catchAllPattern is a special pattern value that matches any path.
// Use this as a fallback/catch-all encoding entry.
const catchAllPattern = "*"

// matchPrefixWithWildcard checks if the target string starts with the pattern,
// where "*" matches any characters within a path segment (delimited by "/").
//
// Wildcard behavior:
//   - A standalone "*" segment matches any single path segment.
//   - "*" within a segment acts as a glob (e.g. "eni-*" matches "eni-0abc123").
//
// Examples:
//
//	matchPrefixWithWildcard("AWSLogs/123/vpcflowlogs/file.gz", "AWSLogs/*/vpcflowlogs") => true
//	matchPrefixWithWildcard("eni-0abc123-all", "eni-*")                                  => true
//	matchPrefixWithWildcard("123_CloudTrail_us-east-1", "*_CloudTrail_*")                => true
//	matchPrefixWithWildcard("any/path", "*")                                             => true
func matchPrefixWithWildcard(target, pattern string) bool {
	if pattern == "" {
		return false
	}

	patternParts := strings.Split(pattern, "/")
	targetParts := strings.Split(target, "/")

	// Target must have at least as many segments as the pattern.
	if len(targetParts) < len(patternParts) {
		return false
	}

	for i, p := range patternParts {
		if p == "*" {
			continue // standalone wildcard matches any single segment
		}
		if strings.Contains(p, "*") {
			// Glob matching within a segment (e.g. "eni-*", "*_CloudTrail_*").
			matched, _ := path.Match(p, targetParts[i])
			if !matched {
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

// comparePatternSpecificity compares two patterns by specificity.
// Returns negative if a is more specific, positive if b is more specific, 0 if equal.
//
// Rules (evaluated left-to-right per segment):
//   - Non-wildcard segment beats a wildcard at the same position.
//   - Longer pattern (more segments) beats a shorter one.
func comparePatternSpecificity(a, b string) int {
	partsA := strings.Split(a, "/")
	partsB := strings.Split(b, "/")

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
//   - 2: exact literal (no wildcards)
//   - 1: glob pattern (partial wildcard, e.g. "eni-*")
//   - 0: full wildcard "*"
func segmentSpecificity(segment string) int {
	if segment == "*" {
		return 0
	}
	if strings.Contains(segment, "*") {
		return 1
	}
	return 2
}

// isCatchAllPattern returns true if the pattern is the bare catch-all "*".
func isCatchAllPattern(pattern string) bool {
	return pattern == catchAllPattern
}
