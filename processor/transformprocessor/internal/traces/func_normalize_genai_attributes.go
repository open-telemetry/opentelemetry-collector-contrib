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

func normalizeGenaiAttributes(span ptrace.Span, lookupTable map[string]genaiTarget, removeOriginals bool) {
	attrs := span.Attributes()

	type rename struct {
		from   string
		target genaiTarget
	}
	renames := make([]rename, 0, len(lookupTable))
	attrs.Range(func(k string, _ pcommon.Value) bool {
		if target, ok := lookupTable[k]; ok {
			renames = append(renames, rename{k, target})
		}
		return true
	})

	if len(renames) == 0 {
		return
	}

	for _, r := range renames {
		if val, ok := attrs.Get(r.from); ok {
			// Skip if target attribute already exists (e.g. multiple source attrs map to same target)
			if _, exists := attrs.Get(r.target.key); exists {
				continue
			}
			if r.target.wrapSlice && val.Type() == pcommon.ValueTypeStr {
				arr := attrs.PutEmptySlice(r.target.key)
				arr.AppendEmpty().SetStr(val.Str())
			} else if val.Type() == pcommon.ValueTypeStr {
				strVal := val.Str()
				if transformed := transformGenaiValue(r.target.key, strVal); transformed != strVal {
					attrs.PutStr(r.target.key, transformed)
				} else {
					val.CopyTo(attrs.PutEmpty(r.target.key))
				}
			} else {
				val.CopyTo(attrs.PutEmpty(r.target.key))
			}
			if removeOriginals {
				attrs.Remove(r.from)
			}
		}
	}
}

// --- Mapping data ---

type genaiMapping struct {
	from      string
	to        string
	wrapSlice bool // when true, wrap scalar string value into a single-element string slice
}

var allProfileNames = []string{"openinference", "openllmetry"}

var profileRegistry = map[string][]genaiMapping{
	// OpenInference: https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md
	"openinference": {
		{from: "llm.token_count.prompt", to: "gen_ai.usage.input_tokens"},
		{from: "llm.token_count.completion", to: "gen_ai.usage.output_tokens"},
		{from: "llm.model_name", to: "gen_ai.request.model"},
		{from: "llm.provider", to: "gen_ai.provider.name"},
		{from: "llm.input_messages", to: "gen_ai.input.messages"},
		{from: "llm.output_messages", to: "gen_ai.output.messages"},
		{from: "embedding.model_name", to: "gen_ai.request.model"},
		{from: "tool.name", to: "gen_ai.tool.name"},
		{from: "tool.description", to: "gen_ai.tool.description"},
		{from: "tool_call.function.arguments", to: "gen_ai.tool.call.arguments"},
		{from: "tool_call.id", to: "gen_ai.tool.call.id"},
		{from: "reranker.model_name", to: "gen_ai.request.model"},
		{from: "agent.name", to: "gen_ai.agent.name"},
		{from: "session.id", to: "gen_ai.conversation.id"},
		{from: "openinference.span.kind", to: "gen_ai.operation.name"},
	},
	// OpenLLMetry: https://www.traceloop.com/docs/openllmetry/contributing/semantic-conventions
	"openllmetry": {
		{from: "llm.usage.prompt_tokens", to: "gen_ai.usage.input_tokens"},
		{from: "llm.usage.completion_tokens", to: "gen_ai.usage.output_tokens"},
		{from: "llm.request.model", to: "gen_ai.request.model"},
		{from: "llm.response.model", to: "gen_ai.response.model"},
		{from: "llm.request.max_tokens", to: "gen_ai.request.max_tokens"},
		{from: "llm.request.temperature", to: "gen_ai.request.temperature"},
		{from: "llm.request.top_p", to: "gen_ai.request.top_p"},
		{from: "llm.top_k", to: "gen_ai.request.top_k"},
		{from: "llm.frequency_penalty", to: "gen_ai.request.frequency_penalty"},
		{from: "llm.presence_penalty", to: "gen_ai.request.presence_penalty"},
		{from: "llm.chat.stop_sequences", to: "gen_ai.request.stop_sequences"},
		{from: "llm.request.functions", to: "gen_ai.tool.definitions"},
		{from: "llm.response.finish_reason", to: "gen_ai.response.finish_reasons", wrapSlice: true},
		{from: "llm.response.stop_reason", to: "gen_ai.response.finish_reasons", wrapSlice: true},
		{from: "llm.request.type", to: "gen_ai.operation.name"},
		{from: "traceloop.span.kind", to: "gen_ai.operation.name"},
		{from: "traceloop.entity.name", to: "gen_ai.agent.name"},
		{from: "traceloop.entity.input", to: "gen_ai.input.messages"},
		{from: "traceloop.entity.output", to: "gen_ai.output.messages"},
	},
}

type genaiTarget struct {
	key       string
	wrapSlice bool
}

// buildGenaiLookupTable merges mappings from the selected profiles into a single lookup table.
// If multiple profiles map the same source attribute to different targets, the last profile wins.
func buildGenaiLookupTable(profiles []string) map[string]genaiTarget {
	table := make(map[string]genaiTarget)
	for _, name := range profiles {
		for _, m := range profileRegistry[name] {
			table[m.from] = genaiTarget{key: m.to, wrapSlice: m.wrapSlice}
		}
	}
	return table
}

// --- Value mappings ---

var operationNameValues = map[string]string{
	// OpenInference span kinds
	"LLM": "chat", "EMBEDDING": "embeddings", "CHAIN": "invoke_agent",
	"RETRIEVER": "retrieval", "RERANKER": "retrieval", "TOOL": "execute_tool",
	"AGENT": "invoke_agent",
	"PROMPT": "text_completion",
	// OpenLLMetry traceloop.span.kind
	"workflow": "invoke_agent", "task": "invoke_agent", "agent": "invoke_agent", "tool": "execute_tool",
	// OpenLLMetry llm.request.type
	"completion": "text_completion", "chat": "chat", "rerank": "retrieval", "embedding": "embeddings",
}

// operationNameValuesNormalized is built at init with lowercased keys for case-insensitive lookup.
var operationNameValuesNormalized = func() map[string]string {
	m := make(map[string]string, len(operationNameValues))
	for k, v := range operationNameValues {
		m[strings.ToLower(k)] = v
	}
	return m
}()

func transformGenaiValue(targetKey string, value string) string {
	if targetKey != "gen_ai.operation.name" {
		return value
	}
	if mapped, ok := operationNameValuesNormalized[strings.ToLower(value)]; ok {
		return mapped
	}
	return value
}
