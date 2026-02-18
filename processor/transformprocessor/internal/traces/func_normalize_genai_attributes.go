// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

const supportedGenAISemconvVersion = "1.39.0"

type normalizeGenaiAttributesArguments struct {
	SemconvVersion  string
	Profiles        ottl.Optional[[]string]
	RemoveOriginals ottl.Optional[bool]
}

func NewNormalizeGenaiAttributesFactory() ottl.Factory[*ottlspan.TransformContext] {
	return ottl.NewFactory("normalize_genai_attributes", &normalizeGenaiAttributesArguments{}, createNormalizeGenaiAttributesFunction)
}

func createNormalizeGenaiAttributesFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottlspan.TransformContext], error) {
	args, ok := oArgs.(*normalizeGenaiAttributesArguments)
	if !ok {
		return nil, errors.New("args must be of type *normalizeGenaiAttributesArguments")
	}

	if args.SemconvVersion != supportedGenAISemconvVersion {
		return nil, fmt.Errorf("unsupported semconv version: %s, supported version: %s", args.SemconvVersion, supportedGenAISemconvVersion)
	}

	// Determine which profiles to use
	var profileNames []string
	if args.Profiles.IsEmpty() {
		profileNames = allProfileNames
	} else {
		profileNames = args.Profiles.Get()
		for _, p := range profileNames {
			if _, ok := profileRegistry[p]; !ok {
				return nil, fmt.Errorf("unknown profile: %q, supported profiles: %v", p, allProfileNames)
			}
		}
	}

	removeOriginals := args.RemoveOriginals.GetOr(false)
	lookupTable := buildGenaiLookupTable(profileNames)

	return func(_ context.Context, tCtx *ottlspan.TransformContext) (any, error) {
		normalizeGenaiAttributes(tCtx.GetSpan(), lookupTable, removeOriginals)
		return nil, nil
	}, nil
}

func normalizeGenaiAttributes(span ptrace.Span, lookupTable map[string]string, removeOriginals bool) {
	attrs := span.Attributes()

	var renames []struct{ from, to string }
	attrs.Range(func(k string, _ pcommon.Value) bool {
		if target, ok := lookupTable[k]; ok {
			renames = append(renames, struct{ from, to string }{k, target})
		}
		return true
	})

	for _, r := range renames {
		if val, ok := attrs.Get(r.from); ok {
			if val.Type() == pcommon.ValueTypeStr {
				if transformed := transformGenaiValue(r.to, val.Str()); transformed != val.Str() {
					attrs.PutStr(r.to, transformed)
				} else {
					val.CopyTo(attrs.PutEmpty(r.to))
				}
			} else {
				val.CopyTo(attrs.PutEmpty(r.to))
			}
			if removeOriginals {
				attrs.Remove(r.from)
			}
		}
	}
}

// --- Mapping data ---

type genaiMapping struct {
	from string
	to   string
}

var allProfileNames = []string{"openinference", "openllmetry", "langchain", "crewai", "pydanticai"}

var profileRegistry = map[string][]genaiMapping{
	"openinference": {
		{"llm.token_count.prompt", "gen_ai.usage.input_tokens"},
		{"llm.token_count.completion", "gen_ai.usage.output_tokens"},
		{"llm.model_name", "gen_ai.request.model"},
		{"llm.provider", "gen_ai.provider.name"},
		{"llm.system", "gen_ai.system"},
		{"llm.input_messages", "gen_ai.input.messages"},
		{"llm.output_messages", "gen_ai.output.messages"},
		{"embedding.model_name", "gen_ai.request.model"},
		{"tool.name", "gen_ai.tool.name"},
		{"tool.description", "gen_ai.tool.description"},
		{"tool_call.function.arguments", "gen_ai.tool.call.arguments"},
		{"tool_call.id", "gen_ai.tool.call.id"},
		{"reranker.model_name", "gen_ai.request.model"},
		{"agent.name", "gen_ai.agent.name"},
		{"session.id", "gen_ai.conversation.id"},
		{"openinference.span.kind", "gen_ai.operation.name"},
	},
	"openllmetry": {
		{"llm.usage.prompt_tokens", "gen_ai.usage.input_tokens"},
		{"llm.usage.completion_tokens", "gen_ai.usage.output_tokens"},
		{"llm.request.model", "gen_ai.request.model"},
		{"llm.response.model", "gen_ai.response.model"},
		{"llm.request.max_tokens", "gen_ai.request.max_tokens"},
		{"llm.request.temperature", "gen_ai.request.temperature"},
		{"llm.request.top_p", "gen_ai.request.top_p"},
		{"llm.top_k", "gen_ai.request.top_k"},
		{"llm.frequency_penalty", "gen_ai.request.frequency_penalty"},
		{"llm.presence_penalty", "gen_ai.request.presence_penalty"},
		{"llm.chat.stop_sequences", "gen_ai.request.stop_sequences"},
		{"llm.request.functions", "gen_ai.tool.definitions"},
		{"llm.response.finish_reason", "gen_ai.response.finish_reasons"},
		{"llm.response.stop_reason", "gen_ai.response.finish_reasons"},
		{"llm.request.type", "gen_ai.operation.name"},
		{"traceloop.span.kind", "gen_ai.operation.name"},
		{"traceloop.entity.name", "gen_ai.agent.name"},
		{"traceloop.entity.input", "gen_ai.input.messages"},
		{"traceloop.entity.output", "gen_ai.output.messages"},
		{"traceloop.association.properties", "gen_ai.conversation.id"},
	},
	"langchain": {
		{"lc.metadata.thread_id", "gen_ai.conversation.id"},
		{"langgraph.node.name", "gen_ai.agent.name"},
	},
	"crewai": {
		{"agent_role", "gen_ai.agent.name"},
		{"crew_id", "gen_ai.conversation.id"},
	},
	"pydanticai": {
		{"agent_name", "gen_ai.agent.name"},
	},
}

func buildGenaiLookupTable(profiles []string) map[string]string {
	table := make(map[string]string)
	for _, name := range profiles {
		for _, m := range profileRegistry[name] {
			table[m.from] = m.to
		}
	}
	return table
}

// --- Value mappings ---

var operationNameValues = map[string]string{
	// OpenInference span kinds
	"LLM": "chat", "EMBEDDING": "embeddings", "CHAIN": "invoke_agent",
	"RETRIEVER": "retrieval", "RERANKER": "retrieval", "TOOL": "execute_tool",
	"AGENT": "invoke_agent", "GUARDRAIL": "guardrail", "EVALUATOR": "evaluate",
	"PROMPT": "text_completion",
	// OpenLLMetry traceloop.span.kind
	"workflow": "invoke_agent", "task": "invoke_agent", "agent": "invoke_agent", "tool": "execute_tool",
	// OpenLLMetry llm.request.type
	"completion": "text_completion", "chat": "chat", "rerank": "retrieval", "embedding": "embeddings",
}

func transformGenaiValue(targetKey string, value string) string {
	if targetKey != "gen_ai.operation.name" {
		return value
	}
	if mapped, ok := operationNameValues[value]; ok {
		return mapped
	}
	if mapped, ok := operationNameValues[strings.ToUpper(value)]; ok {
		return mapped
	}
	if mapped, ok := operationNameValues[strings.ToLower(value)]; ok {
		return mapped
	}
	return value
}
