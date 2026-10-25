// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelsemconv pins the OTel semantic-conventions version this
// processor targets and declares the gen_ai.* attribute keys it emits along
// with their spec types.
//
// The gen_ai.* conventions moved to
// https://github.com/open-telemetry/semantic-conventions-genai in semconv
// v1.42.0 and are no longer generated into go.opentelemetry.io/otel/semconv,
// so the keys are declared here and only SchemaURL comes from that package.
package otelsemconv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"

import (
	"reflect"

	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// SchemaURL is the OTel semconv schema URL for the targeted version.
const SchemaURL = conventions.SchemaURL

// targetTypes maps each key registered via typed to its spec type. Keys with
// spec type "any" are not registered and pass through Coerce verbatim.
var targetTypes = map[string]reflect.Type{}

func typed[T any](key string) string {
	targetTypes[key] = reflect.TypeFor[T]()
	return key
}

//nolint:gochecknoglobals // canonical key registry
var (
	// gen_ai.agent.*
	GenAIAgentName = typed[string]("gen_ai.agent.name")

	// gen_ai.conversation.*
	GenAIConversationID = typed[string]("gen_ai.conversation.id")

	// gen_ai.input.*
	GenAIInputMessages = "gen_ai.input.messages"

	// gen_ai.operation.*
	GenAIOperationName = "gen_ai.operation.name"

	// gen_ai.output.*
	GenAIOutputMessages = "gen_ai.output.messages"

	// gen_ai.provider.*
	GenAIProviderName = "gen_ai.provider.name"

	// gen_ai.request.*
	GenAIRequestFrequencyPenalty = typed[float64]("gen_ai.request.frequency_penalty")
	GenAIRequestMaxTokens        = typed[int64]("gen_ai.request.max_tokens")
	GenAIRequestModel            = typed[string]("gen_ai.request.model")
	GenAIRequestPresencePenalty  = typed[float64]("gen_ai.request.presence_penalty")
	GenAIRequestStopSequences    = typed[[]string]("gen_ai.request.stop_sequences")
	GenAIRequestTemperature      = typed[float64]("gen_ai.request.temperature")
	GenAIRequestTopK             = typed[float64]("gen_ai.request.top_k")
	GenAIRequestTopP             = typed[float64]("gen_ai.request.top_p")

	// gen_ai.response.*
	GenAIResponseFinishReasons = typed[[]string]("gen_ai.response.finish_reasons")
	GenAIResponseModel         = typed[string]("gen_ai.response.model")

	// gen_ai.tool.*
	GenAIToolCallArguments = "gen_ai.tool.call.arguments"
	GenAIToolCallID        = typed[string]("gen_ai.tool.call.id")
	GenAIToolDefinitions   = "gen_ai.tool.definitions"
	GenAIToolDescription   = typed[string]("gen_ai.tool.description")
	GenAIToolName          = typed[string]("gen_ai.tool.name")

	// gen_ai.usage.*
	GenAIUsageInputTokens  = typed[int64]("gen_ai.usage.input_tokens")
	GenAIUsageOutputTokens = typed[int64]("gen_ai.usage.output_tokens")
)

// gen_ai.operation.name enum values.
const (
	GenAIOperationNameChat           = "chat"
	GenAIOperationNameEmbeddings     = "embeddings"
	GenAIOperationNameExecuteTool    = "execute_tool"
	GenAIOperationNameInvokeAgent    = "invoke_agent"
	GenAIOperationNameRetrieval      = "retrieval"
	GenAIOperationNameTextCompletion = "text_completion"
	GenAIOperationNameInvokeWorkflow = "invoke_workflow"
)
