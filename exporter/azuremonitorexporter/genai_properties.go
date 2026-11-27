// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"fmt"
	"unicode/utf8"
)

// genAIContentPropertyMaxLength is the length GenAI content properties are
// truncated to. Application Insights stores them in the AppGenAIContent table,
// and the Azure Monitor SDK exporters cap them at 256 KiB instead of the
// 8192 characters used for other properties.
const genAIContentPropertyMaxLength = 256 * 1024

// genAIContentAttributes are the GenAI content attributes that keep the larger limit.
var genAIContentAttributes = []string{
	"gen_ai.input.messages",
	"gen_ai.output.messages",
	"gen_ai.system_instructions",
	"gen_ai.tool.definitions",
	"gen_ai.tool.call.arguments",
	"gen_ai.tool.call.result",
	"gen_ai.evaluation.explanation",
}

// sanitizeKeepingGenAIContent runs sanitizeFunc, which truncates every value in
// properties to 8192 characters, while holding the GenAI content properties
// back. Those are truncated to genAIContentPropertyMaxLength instead.
func sanitizeKeepingGenAIContent(properties map[string]string, sanitizeFunc func() []string) []string {
	held := map[string]string{}
	for _, key := range genAIContentAttributes {
		if value, ok := properties[key]; ok {
			held[key] = value
			delete(properties, key)
		}
	}

	warnings := sanitizeFunc()

	for key, value := range held {
		if len(value) > genAIContentPropertyMaxLength {
			value = truncateUTF8(value, genAIContentPropertyMaxLength)
			warnings = append(warnings, fmt.Sprintf("Properties has value with length exceeding max of %d: %s", genAIContentPropertyMaxLength, key))
		}
		properties[key] = value
	}
	return warnings
}

// truncateUTF8 cuts s to at most maxLen bytes without splitting a UTF-8 character.
func truncateUTF8(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := maxLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
