// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

// mapping defines a source attribute key and its normalized target.
type mapping struct {
	from      string
	to        string
	wrapSlice bool // when true, wrap scalar string value into a single-element string slice
}

// mappingTarget holds the resolved target for a source attribute.
type mappingTarget struct {
	key       string
	wrapSlice bool
}

// builtInMappings returns the built-in mappings for a given source.
// SourceCustom has no built-in mappings.
func builtInMappings(name SourceName) []mapping {
	switch name {
	case SourceOpenInference:
		return openInferenceMappings
	case SourceOpenLLMetry:
		return openLLMetryMappings
	}
	return nil
}

// buildLookupTable assembles a per-source lookup from built-in mappings
// plus the source's user-defined custom mappings. Custom mappings override
// built-in mappings on conflict.
func buildLookupTable(name SourceName, customMappings map[string]string) map[string]mappingTarget {
	table := make(map[string]mappingTarget)
	for _, m := range builtInMappings(name) {
		table[m.from] = mappingTarget{key: m.to, wrapSlice: m.wrapSlice}
	}
	for src, dst := range customMappings {
		table[src] = mappingTarget{key: dst}
	}
	return table
}

// openInferenceMappings is the OpenInference attribute-rename table.
// Reference: https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md
var openInferenceMappings = []mapping{
	// Token usage
	{from: "llm.token_count.prompt", to: "gen_ai.usage.input_tokens"},
	{from: "llm.token_count.completion", to: "gen_ai.usage.output_tokens"},

	// Model & provider
	{from: "llm.model_name", to: "gen_ai.request.model"},
	{from: "llm.provider", to: "gen_ai.provider.name"},

	// Input/output content
	{from: "llm.input_messages", to: "gen_ai.input.messages"},
	{from: "llm.output_messages", to: "gen_ai.output.messages"},

	// Embeddings
	{from: "embedding.model_name", to: "gen_ai.request.model"},

	// Tool
	{from: "tool.name", to: "gen_ai.tool.name"},
	{from: "tool.description", to: "gen_ai.tool.description"},
	{from: "tool_call.function.arguments", to: "gen_ai.tool.call.arguments"},
	{from: "tool_call.id", to: "gen_ai.tool.call.id"},

	// Reranker model (mutually exclusive with llm/embedding model)
	{from: "reranker.model_name", to: "gen_ai.request.model"},

	// Agent & session
	{from: "agent.name", to: "gen_ai.agent.name"},
	{from: "session.id", to: "gen_ai.conversation.id"},

	// Span kind -> operation name (value mapping handled separately)
	{from: "openinference.span.kind", to: "gen_ai.operation.name"},
}

// openLLMetryMappings is the OpenLLMetry (Traceloop) attribute-rename table.
// Reference: https://www.traceloop.com/docs/openllmetry/contributing/semantic-conventions
var openLLMetryMappings = []mapping{
	// Token usage
	{from: "llm.usage.prompt_tokens", to: "gen_ai.usage.input_tokens"},
	{from: "llm.usage.completion_tokens", to: "gen_ai.usage.output_tokens"},

	// Model & provider
	{from: "llm.request.model", to: "gen_ai.request.model"},
	{from: "llm.response.model", to: "gen_ai.response.model"},

	// Request params
	{from: "llm.request.max_tokens", to: "gen_ai.request.max_tokens"},
	{from: "llm.request.temperature", to: "gen_ai.request.temperature"},
	{from: "llm.request.top_p", to: "gen_ai.request.top_p"},
	{from: "llm.top_k", to: "gen_ai.request.top_k"},
	{from: "llm.frequency_penalty", to: "gen_ai.request.frequency_penalty"},
	{from: "llm.presence_penalty", to: "gen_ai.request.presence_penalty"},
	{from: "llm.chat.stop_sequences", to: "gen_ai.request.stop_sequences"},
	{from: "llm.request.functions", to: "gen_ai.tool.definitions"},

	// Response: string to string[] conversion
	{from: "llm.response.finish_reason", to: "gen_ai.response.finish_reasons", wrapSlice: true},
	{from: "llm.response.stop_reason", to: "gen_ai.response.finish_reasons", wrapSlice: true},

	// Operation: llm.request.type on LLM spans, traceloop.span.kind on workflow spans
	{from: "llm.request.type", to: "gen_ai.operation.name"},
	{from: "traceloop.span.kind", to: "gen_ai.operation.name"},

	// Traceloop workflow/entity (agentic)
	{from: "traceloop.entity.name", to: "gen_ai.agent.name"},
	{from: "traceloop.entity.input", to: "gen_ai.input.messages"},
	{from: "traceloop.entity.output", to: "gen_ai.output.messages"},
}
