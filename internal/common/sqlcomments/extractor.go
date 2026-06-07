// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlcomments // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sqlcomments"

import (
	"regexp"
	"strings"
)

var (
	leadingBlockCommentRegex = regexp.MustCompile(`^\s*(/\*.*?\*/\s*)+`)
	commentContentRegex      = regexp.MustCompile(`/\*(.*?)\*/`)
)

// ExtractAndFilterComments returns the comma-separated key=value pairs found in
// the leading /* */ block comments of sqlText whose keys are in allowedKeys.
// Extraction is disabled (returns "") when allowedKeys is empty.
func ExtractAndFilterComments(sqlText string, allowedKeys []string) string {
	if len(allowedKeys) == 0 {
		return ""
	}

	values := parseLeadingComments(sqlText)

	var filteredPairs []string
	for _, key := range allowedKeys {
		if value, ok := values[key]; ok {
			filteredPairs = append(filteredPairs, key+"="+value)
		}
	}

	return strings.Join(filteredPairs, ",")
}

func parseLeadingComments(sqlText string) map[string]string {
	values := make(map[string]string)

	leading := leadingBlockCommentRegex.FindString(sqlText)
	if leading == "" {
		return values
	}

	for _, commentMatch := range commentContentRegex.FindAllStringSubmatch(leading, -1) {
		if len(commentMatch) > 1 {
			addPairs(values, commentMatch[1])
		}
	}
	return values
}

func addPairs(values map[string]string, content string) {
	for pair := range strings.SplitSeq(content, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		keyValue := strings.SplitN(pair, "=", 2)
		if len(keyValue) != 2 {
			continue
		}

		key := strings.TrimSpace(keyValue[0])
		if _, ok := values[key]; !ok {
			values[key] = strings.TrimSpace(keyValue[1])
		}
	}
}
