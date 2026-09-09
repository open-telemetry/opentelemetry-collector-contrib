// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsemconv

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultTargetMatchesSchemaURL checks that the conventions import (via
// SchemaURL), defaultTarget, and the generated table all name the same release.
func TestDefaultTargetMatchesSchemaURL(t *testing.T) {
	assert.Truef(t, strings.HasSuffix(SchemaURL, "/"+strings.TrimPrefix(defaultTarget, "v")),
		"SchemaURL %q does not name defaultTarget %q", SchemaURL, defaultTarget)

	_, ok := typesByTarget[defaultTarget]
	assert.Truef(t, ok, "no generated table for defaultTarget %q", defaultTarget)
}

// TestTargetTypesCoversDeclaredKeys checks that every key this package exports
// is defined by the targeted registry. Coerce passes unknown keys through, so a
// key renamed upstream would otherwise go unenforced.
func TestTargetTypesCoversDeclaredKeys(t *testing.T) {
	declared := map[string]string{
		"GenAIAgentName":               GenAIAgentName,
		"GenAIConversationID":          GenAIConversationID,
		"GenAIInputMessages":           GenAIInputMessages,
		"GenAIOperationName":           GenAIOperationName,
		"GenAIOutputMessages":          GenAIOutputMessages,
		"GenAIProviderName":            GenAIProviderName,
		"GenAIRequestFrequencyPenalty": GenAIRequestFrequencyPenalty,
		"GenAIRequestMaxTokens":        GenAIRequestMaxTokens,
		"GenAIRequestModel":            GenAIRequestModel,
		"GenAIRequestPresencePenalty":  GenAIRequestPresencePenalty,
		"GenAIRequestStopSequences":    GenAIRequestStopSequences,
		"GenAIRequestTemperature":      GenAIRequestTemperature,
		"GenAIRequestTopK":             GenAIRequestTopK,
		"GenAIRequestTopP":             GenAIRequestTopP,
		"GenAIResponseFinishReasons":   GenAIResponseFinishReasons,
		"GenAIResponseModel":           GenAIResponseModel,
		"GenAIToolCallArguments":       GenAIToolCallArguments,
		"GenAIToolCallID":              GenAIToolCallID,
		"GenAIToolDefinitions":         GenAIToolDefinitions,
		"GenAIToolDescription":         GenAIToolDescription,
		"GenAIToolName":                GenAIToolName,
		"GenAIUsageInputTokens":        GenAIUsageInputTokens,
		"GenAIUsageOutputTokens":       GenAIUsageOutputTokens,
	}
	for name, key := range declared {
		_, ok := targetTypes[key]
		assert.Truef(t, ok, "%s (%q) is not defined by the %s registry", name, key, defaultTarget)
	}
}

// TestTargetTypesMatchesSemconvConstructors pins the type of every key this
// package exports, cross-checked against the typed constructors in the semconv
// Go library.
func TestTargetTypesMatchesSemconvConstructors(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want kind
	}{
		{GenAIAgentName, kindString},
		{GenAIConversationID, kindString},
		{GenAIRequestFrequencyPenalty, kindDouble},
		{GenAIRequestMaxTokens, kindInt},
		{GenAIRequestModel, kindString},
		{GenAIRequestPresencePenalty, kindDouble},
		{GenAIRequestStopSequences, kindStringSlice},
		{GenAIRequestTemperature, kindDouble},
		{GenAIRequestTopK, kindDouble},
		{GenAIRequestTopP, kindDouble},
		{GenAIResponseFinishReasons, kindStringSlice},
		{GenAIResponseModel, kindString},
		{GenAIToolCallID, kindString},
		{GenAIToolDescription, kindString},
		{GenAIToolName, kindString},
		{GenAIUsageInputTokens, kindInt},
		{GenAIUsageOutputTokens, kindInt},

		// Document-shaped: registry type "any", passed through untouched.
		{GenAIInputMessages, kindAny},
		{GenAIOutputMessages, kindAny},
		{GenAIToolCallArguments, kindAny},
		{GenAIToolDefinitions, kindAny},

		// Enums. No typed constructor in the semconv Go library, so these are
		// checked against the registry rather than against a constructor.
		{GenAIOperationName, kindString},
		{GenAIProviderName, kindString},
	} {
		t.Run(tc.key, func(t *testing.T) {
			assert.Equal(t, tc.want, targetTypes[tc.key])
		})
	}
}
