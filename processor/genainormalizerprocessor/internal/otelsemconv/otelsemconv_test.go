// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsemconv

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0" //nolint:depguard // drift-guard tests assert against upstream semconv symbols
)

func TestDriftGuard_SchemaURL(t *testing.T) {
	assert.Equal(t, conventions.SchemaURL, SchemaURL)
}

func TestDriftGuard_AttributeKeys(t *testing.T) {
	tests := []struct {
		name     string
		local    string
		upstream string
	}{
		// gen_ai.agent.*
		{"GenAIAgentName", GenAIAgentName, string(conventions.GenAIAgentNameKey)},

		// gen_ai.conversation.*
		{"GenAIConversationID", GenAIConversationID, string(conventions.GenAIConversationIDKey)},

		// gen_ai.input.*
		{"GenAIInputMessages", GenAIInputMessages, string(conventions.GenAIInputMessagesKey)},

		// gen_ai.operation.*
		{"GenAIOperationName", GenAIOperationName, string(conventions.GenAIOperationNameKey)},

		// gen_ai.output.*
		{"GenAIOutputMessages", GenAIOutputMessages, string(conventions.GenAIOutputMessagesKey)},

		// gen_ai.provider.*
		{"GenAIProviderName", GenAIProviderName, string(conventions.GenAIProviderNameKey)},

		// gen_ai.request.*
		{"GenAIRequestFrequencyPenalty", GenAIRequestFrequencyPenalty, string(conventions.GenAIRequestFrequencyPenaltyKey)},
		{"GenAIRequestMaxTokens", GenAIRequestMaxTokens, string(conventions.GenAIRequestMaxTokensKey)},
		{"GenAIRequestModel", GenAIRequestModel, string(conventions.GenAIRequestModelKey)},
		{"GenAIRequestPresencePenalty", GenAIRequestPresencePenalty, string(conventions.GenAIRequestPresencePenaltyKey)},
		{"GenAIRequestStopSequences", GenAIRequestStopSequences, string(conventions.GenAIRequestStopSequencesKey)},
		{"GenAIRequestTemperature", GenAIRequestTemperature, string(conventions.GenAIRequestTemperatureKey)},
		{"GenAIRequestTopK", GenAIRequestTopK, string(conventions.GenAIRequestTopKKey)},
		{"GenAIRequestTopP", GenAIRequestTopP, string(conventions.GenAIRequestTopPKey)},

		// gen_ai.response.*
		{"GenAIResponseFinishReasons", GenAIResponseFinishReasons, string(conventions.GenAIResponseFinishReasonsKey)},
		{"GenAIResponseModel", GenAIResponseModel, string(conventions.GenAIResponseModelKey)},

		// gen_ai.tool.*
		{"GenAIToolCallArguments", GenAIToolCallArguments, string(conventions.GenAIToolCallArgumentsKey)},
		{"GenAIToolCallID", GenAIToolCallID, string(conventions.GenAIToolCallIDKey)},
		{"GenAIToolDefinitions", GenAIToolDefinitions, string(conventions.GenAIToolDefinitionsKey)},
		{"GenAIToolDescription", GenAIToolDescription, string(conventions.GenAIToolDescriptionKey)},
		{"GenAIToolName", GenAIToolName, string(conventions.GenAIToolNameKey)},

		// gen_ai.usage.*
		{"GenAIUsageInputTokens", GenAIUsageInputTokens, string(conventions.GenAIUsageInputTokensKey)},
		{"GenAIUsageOutputTokens", GenAIUsageOutputTokens, string(conventions.GenAIUsageOutputTokensKey)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.upstream, tt.local)
		})
	}
}

func TestDriftGuard_OperationNameValues(t *testing.T) {
	tests := []struct {
		name     string
		local    string
		upstream string
	}{
		{"GenAIOperationNameChat", GenAIOperationNameChat, conventions.GenAIOperationNameChat.Value.AsString()},
		{"GenAIOperationNameEmbeddings", GenAIOperationNameEmbeddings, conventions.GenAIOperationNameEmbeddings.Value.AsString()},
		{"GenAIOperationNameExecuteTool", GenAIOperationNameExecuteTool, conventions.GenAIOperationNameExecuteTool.Value.AsString()},
		{"GenAIOperationNameInvokeAgent", GenAIOperationNameInvokeAgent, conventions.GenAIOperationNameInvokeAgent.Value.AsString()},
		{"GenAIOperationNameRetrieval", GenAIOperationNameRetrieval, conventions.GenAIOperationNameRetrieval.Value.AsString()},
		{"GenAIOperationNameTextCompletion", GenAIOperationNameTextCompletion, conventions.GenAIOperationNameTextCompletion.Value.AsString()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.upstream, tt.local)
		})
	}
}

func TestDriftGuard_TargetTypes(t *testing.T) {
	ctors := map[string]any{
		GenAIAgentName:               conventions.GenAIAgentName,
		GenAIConversationID:          conventions.GenAIConversationID,
		GenAIRequestFrequencyPenalty: conventions.GenAIRequestFrequencyPenalty,
		GenAIRequestMaxTokens:        conventions.GenAIRequestMaxTokens,
		GenAIRequestModel:            conventions.GenAIRequestModel,
		GenAIRequestPresencePenalty:  conventions.GenAIRequestPresencePenalty,
		GenAIRequestStopSequences:    conventions.GenAIRequestStopSequences,
		GenAIRequestTemperature:      conventions.GenAIRequestTemperature,
		GenAIRequestTopK:             conventions.GenAIRequestTopK,
		GenAIRequestTopP:             conventions.GenAIRequestTopP,
		GenAIResponseFinishReasons:   conventions.GenAIResponseFinishReasons,
		GenAIResponseModel:           conventions.GenAIResponseModel,
		GenAIToolCallID:              conventions.GenAIToolCallID,
		GenAIToolDescription:         conventions.GenAIToolDescription,
		GenAIToolName:                conventions.GenAIToolName,
		GenAIUsageInputTokens:        conventions.GenAIUsageInputTokens,
		GenAIUsageOutputTokens:       conventions.GenAIUsageOutputTokens,
	}
	assert.Len(t, targetTypes, len(ctors))
	for key, ctor := range ctors {
		t.Run(key, func(t *testing.T) {
			// For variadic ctors, In(0) is already the slice type.
			assert.Equal(t, reflect.TypeOf(ctor).In(0), targetTypes[key])
		})
	}
}
