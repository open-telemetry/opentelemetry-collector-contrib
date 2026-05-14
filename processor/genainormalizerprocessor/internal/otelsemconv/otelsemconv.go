// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelsemconv is the single source of truth for the OTel semantic-
// conventions version this processor targets. Bumping versions: update the
// import path below, verify referenced constants still exist, and bump any
// version references in public-facing docs.
package otelsemconv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"

import conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

// SchemaURL is the OTel semconv schema URL for the targeted version.
const SchemaURL = conventions.SchemaURL

// GenAI attribute keys as plain strings for use as pdata keys.
// Grouped by top-level subnamespace (gen_ai.<group>.*), alphabetical within group.
var (
	// gen_ai.agent.*
	GenAIAgentName = string(conventions.GenAIAgentNameKey)

	// gen_ai.conversation.*
	GenAIConversationID = string(conventions.GenAIConversationIDKey)

	// gen_ai.input.*
	GenAIInputMessages = string(conventions.GenAIInputMessagesKey)

	// gen_ai.operation.*
	GenAIOperationName = string(conventions.GenAIOperationNameKey)

	// gen_ai.output.*
	GenAIOutputMessages = string(conventions.GenAIOutputMessagesKey)

	// gen_ai.provider.*
	GenAIProviderName = string(conventions.GenAIProviderNameKey)

	// gen_ai.request.*
	GenAIRequestFrequencyPenalty = string(conventions.GenAIRequestFrequencyPenaltyKey)
	GenAIRequestMaxTokens        = string(conventions.GenAIRequestMaxTokensKey)
	GenAIRequestModel            = string(conventions.GenAIRequestModelKey)
	GenAIRequestPresencePenalty  = string(conventions.GenAIRequestPresencePenaltyKey)
	GenAIRequestStopSequences    = string(conventions.GenAIRequestStopSequencesKey)
	GenAIRequestTemperature      = string(conventions.GenAIRequestTemperatureKey)
	GenAIRequestTopK             = string(conventions.GenAIRequestTopKKey)
	GenAIRequestTopP             = string(conventions.GenAIRequestTopPKey)

	// gen_ai.response.*
	GenAIResponseFinishReasons = string(conventions.GenAIResponseFinishReasonsKey)
	GenAIResponseModel         = string(conventions.GenAIResponseModelKey)

	// gen_ai.tool.*
	GenAIToolCallArguments = string(conventions.GenAIToolCallArgumentsKey)
	GenAIToolCallID        = string(conventions.GenAIToolCallIDKey)
	GenAIToolDefinitions   = string(conventions.GenAIToolDefinitionsKey)
	GenAIToolDescription   = string(conventions.GenAIToolDescriptionKey)
	GenAIToolName          = string(conventions.GenAIToolNameKey)

	// gen_ai.usage.*
	GenAIUsageInputTokens  = string(conventions.GenAIUsageInputTokensKey)
	GenAIUsageOutputTokens = string(conventions.GenAIUsageOutputTokensKey)
)
