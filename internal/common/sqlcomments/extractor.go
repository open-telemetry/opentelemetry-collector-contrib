// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlcomments // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sqlcomments"

import (
	"regexp"
	"strings"
)

var (
	// leadingBlockCommentRegex matches one or more leading /* */ block comments
	leadingBlockCommentRegex = regexp.MustCompile(`^\s*(/\*.*?\*/\s*)+`)
	// commentContentRegex extracts content between /* and */ delimiters
	commentContentRegex = regexp.MustCompile(`/\*(.*?)\*/`)
)

// ExtractAndFilterComments extracts leading /* */ block comments from SQL,
// parses them as key=value pairs, and returns only allowed keys.
// Returns comma-separated filtered pairs, or empty string if no allowed keys found.
// Format: "key1=value1,key2=value2"
//
// This function is designed to be secure by default:
// - If allowedKeys is empty or nil, returns empty string (no extraction)
// - Only keys explicitly listed in allowedKeys are included in the result
// - Duplicate keys use first occurrence only
// - Malformed pairs (without =) are silently skipped
//
// Example:
//
//	sqlText := "/* key1=value1,key2=value2 */ SELECT * FROM users"
//	allowedKeys := []string{"key1"}
//	result := ExtractAndFilterComments(sqlText, allowedKeys)
//	// result == "key1=value1"
func ExtractAndFilterComments(sqlText string, allowedKeys []string) string {
	// Early exit: if no allowed keys, return empty immediately (secure by default)
	if len(allowedKeys) == 0 {
		return ""
	}

	// Extract leading block comments using regex
	matches := leadingBlockCommentRegex.FindString(sqlText)
	if matches == "" {
		return ""
	}

	// Strip /* and */ delimiters from all comments
	// Match each individual comment block
	commentMatches := commentContentRegex.FindAllStringSubmatch(matches, -1)
	if len(commentMatches) == 0 {
		return ""
	}

	// Concatenate all comment contents
	var allComments strings.Builder
	for i, match := range commentMatches {
		if len(match) > 1 {
			if i > 0 {
				allComments.WriteString(",")
			}
			allComments.WriteString(strings.TrimSpace(match[1]))
		}
	}

	commentContent := allComments.String()
	if commentContent == "" {
		return ""
	}

	// Parse key=value pairs and filter by allowed keys
	pairs := strings.Split(commentContent, ",")
	var filteredPairs []string
	seenKeys := make(map[string]bool)

	// Create a set of allowed keys for O(1) lookup
	allowedSet := make(map[string]bool)
	for _, key := range allowedKeys {
		allowedSet[key] = true
	}

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split by first = only
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			// Malformed pair, skip it
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Check if key is allowed and not already seen (use first occurrence)
		if allowedSet[key] && !seenKeys[key] {
			seenKeys[key] = true
			filteredPairs = append(filteredPairs, key+"="+value)
		}
	}

	// Join filtered pairs with comma (no space)
	return strings.Join(filteredPairs, ",")
}
