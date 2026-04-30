package genainormalizerprocessor

// Mapping defines a source attribute key and its normalized target.
type Mapping struct {
	From      string
	To        string
	WrapSlice bool // when true, wrap scalar string value into a single-element string slice
}

// MappingTarget holds the resolved target for a source attribute.
type MappingTarget struct {
	Key       string
	WrapSlice bool
}

// Profile returns the attribute mappings for a given profile name.
func Profile(name string) []Mapping {
	switch name {
	case "openinference":
		return openInferenceMappings
	case "openllmetry":
		return openLLMetryMappings
	default:
		return nil
	}
}

// BuildLookupTable merges enabled profiles into a single map for O(1) lookup.
// If multiple profiles map the same source attribute to different targets, the last profile wins.
func BuildLookupTable(profiles []string) map[string]MappingTarget {
	table := make(map[string]MappingTarget)
	for _, name := range profiles {
		for _, m := range Profile(name) {
			table[m.From] = MappingTarget{Key: m.To, WrapSlice: m.WrapSlice}
		}
	}
	return table
}

// --- OpenInference (Arize) ---
// Ref: https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md

var openInferenceMappings = []Mapping{
	// Token usage
	{From: "llm.token_count.prompt", To: "gen_ai.usage.input_tokens"},
	{From: "llm.token_count.completion", To: "gen_ai.usage.output_tokens"},

	// Model & provider
	{From: "llm.model_name", To: "gen_ai.request.model"},
	{From: "llm.provider", To: "gen_ai.provider.name"},

	// Input/output content
	{From: "llm.input_messages", To: "gen_ai.input.messages"},
	{From: "llm.output_messages", To: "gen_ai.output.messages"},

	// Embeddings
	{From: "embedding.model_name", To: "gen_ai.request.model"},

	// Tool
	{From: "tool.name", To: "gen_ai.tool.name"},
	{From: "tool.description", To: "gen_ai.tool.description"},
	{From: "tool_call.function.arguments", To: "gen_ai.tool.call.arguments"},
	{From: "tool_call.id", To: "gen_ai.tool.call.id"},

	// Reranker model (mutually exclusive with llm/embedding model)
	{From: "reranker.model_name", To: "gen_ai.request.model"},

	// Agent & session
	{From: "agent.name", To: "gen_ai.agent.name"},
	{From: "session.id", To: "gen_ai.conversation.id"},

	// Span kind → operation name (value mapping handled separately)
	{From: "openinference.span.kind", To: "gen_ai.operation.name"},
}

// --- OpenLLMetry (Traceloop) ---
// Ref: https://www.traceloop.com/docs/openllmetry/contributing/semantic-conventions

var openLLMetryMappings = []Mapping{
	// Token usage
	{From: "llm.usage.prompt_tokens", To: "gen_ai.usage.input_tokens"},
	{From: "llm.usage.completion_tokens", To: "gen_ai.usage.output_tokens"},

	// Model & provider
	{From: "llm.request.model", To: "gen_ai.request.model"},
	{From: "llm.response.model", To: "gen_ai.response.model"},

	// Request params
	{From: "llm.request.max_tokens", To: "gen_ai.request.max_tokens"},
	{From: "llm.request.temperature", To: "gen_ai.request.temperature"},
	{From: "llm.request.top_p", To: "gen_ai.request.top_p"},
	{From: "llm.top_k", To: "gen_ai.request.top_k"},
	{From: "llm.frequency_penalty", To: "gen_ai.request.frequency_penalty"},
	{From: "llm.presence_penalty", To: "gen_ai.request.presence_penalty"},
	{From: "llm.chat.stop_sequences", To: "gen_ai.request.stop_sequences"},
	{From: "llm.request.functions", To: "gen_ai.tool.definitions"},

	// Response — string to string[] conversion
	{From: "llm.response.finish_reason", To: "gen_ai.response.finish_reasons", WrapSlice: true},
	{From: "llm.response.stop_reason", To: "gen_ai.response.finish_reasons", WrapSlice: true},

	// Operation — llm.request.type on LLM spans, traceloop.span.kind on workflow spans
	{From: "llm.request.type", To: "gen_ai.operation.name"},
	{From: "traceloop.span.kind", To: "gen_ai.operation.name"},

	// Traceloop workflow/entity (agentic)
	{From: "traceloop.entity.name", To: "gen_ai.agent.name"},
	{From: "traceloop.entity.input", To: "gen_ai.input.messages"},
	{From: "traceloop.entity.output", To: "gen_ai.output.messages"},
}
