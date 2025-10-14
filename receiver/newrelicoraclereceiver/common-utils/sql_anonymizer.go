package commonutils

import (
	"regexp"
	"strings"
)

// AnonymizeAndNormalize anonymizes literal values and normalizes a SQL query.
func AnonymizeAndNormalize(query string) string {
	// Replace numeric literals with a placeholder.
	reNumbers := regexp.MustCompile(`\d+`)
	cleanedQuery := reNumbers.ReplaceAllString(query, "?")

	// Replace single-quoted string literals with a placeholder.
	reSingleQuotes := regexp.MustCompile(`'[^']*'`)
	cleanedQuery = reSingleQuotes.ReplaceAllString(cleanedQuery, "?")

	// Convert to lowercase for normalization.
	cleanedQuery = strings.ToLower(cleanedQuery)

	// Remove semicolons.
	cleanedQuery = strings.ReplaceAll(cleanedQuery, ";", "")

	// Trim leading/trailing whitespace.
	cleanedQuery = strings.TrimSpace(cleanedQuery)

	// Normalize internal whitespace (collapse multiple spaces into single spaces).
	cleanedQuery = strings.Join(strings.Fields(cleanedQuery), " ")

	return cleanedQuery
}