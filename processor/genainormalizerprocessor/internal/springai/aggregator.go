// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package springai // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/springai"

import (
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

const (
	// genAISystem is the deprecated name of gen_ai.provider.name, which Spring
	// AI still writes.
	genAISystem = "gen_ai.system"

	// frameworkSystem is the gen_ai.system value Spring AI sets on its own
	// chat client, advisor and tool spans. It names the framework, not a provider.
	frameworkSystem = "spring_ai"

	// cacheExclusiveSystem reports input_tokens without the cached tokens:
	// https://github.com/spring-projects/spring-ai/blob/cba03cdd219d9ee45d30c691349bfc733f6d1110/models/spring-ai-anthropic/src/main/java/org/springframework/ai/anthropic/AnthropicChatModel.java#L1413-L1419
	cacheExclusiveSystem = "anthropic"
)

// scalarKeys are canonical keys Spring AI writes under their own name, but as
// strings. otelsemconv.Coerce parses them into their semconv types.
var scalarKeys = []string{
	otelsemconv.GenAIUsageInputTokens,
	otelsemconv.GenAIUsageOutputTokens,
	otelsemconv.GenAIUsageCacheCreationInputTokens,
	otelsemconv.GenAIUsageCacheReadInputTokens,
	otelsemconv.GenAIRequestMaxTokens,
	otelsemconv.GenAIRequestTemperature,
	otelsemconv.GenAIRequestTopP,
	otelsemconv.GenAIRequestTopK,
	otelsemconv.GenAIRequestFrequencyPenalty,
	otelsemconv.GenAIRequestPresencePenalty,
}

// listKeys are written by Spring AI's ObservabilityHelper.concatenateStrings,
// which quotes each element without escaping it, so a value is JSON only when
// no element contains a quote.
var listKeys = []string{
	otelsemconv.GenAIResponseFinishReasons,
	otelsemconv.GenAIRequestStopSequences,
}

// providerNames maps Spring AI's AiProvider values onto the
// gen_ai.provider.name enum where the two spell a provider differently.
var providerNames = map[string]string{
	"bedrock_converse": conventions.GenAIProviderNameAWSBedrock.Value.AsString(),
	"vertex_ai":        conventions.GenAIProviderNameGCPVertexAI.Value.AsString(),
	"google_genai":     conventions.GenAIProviderNameGCPGenAI.Value.AsString(),
}

var operationNames = map[string]string{
	"embedding": otelsemconv.GenAIOperationNameEmbeddings,
}

// Aggregator retypes the string values Spring AI writes under canonical
// gen_ai.* keys and derives gen_ai.provider.name from gen_ai.system.
type Aggregator struct{}

// AggregateAttributes implements the processor's attributeAggregator.
func (Aggregator) AggregateAttributes(attrs pcommon.Map, removeOriginals, overwrite bool) bool {
	// Only a count still typed as the bridge's string is folded, so a span
	// normalized twice is not charged the cache tokens twice.
	foldCache := isStr(attrs, otelsemconv.GenAIUsageInputTokens) && strAttr(attrs, genAISystem) == cacheExclusiveSystem

	wrote := false
	for _, key := range scalarKeys {
		wrote = retype(attrs, key, func(src, dst pcommon.Value) bool {
			return otelsemconv.Coerce(key, src, dst)
		}) || wrote
	}
	if foldCache {
		includeCachedInputTokens(attrs)
	}
	for _, key := range listKeys {
		wrote = retype(attrs, key, parseList) || wrote
	}
	wrote = putProviderName(attrs, removeOriginals, overwrite) || wrote
	return foldOperationName(attrs) || wrote
}

func retype(attrs pcommon.Map, key string, parse func(src, dst pcommon.Value) bool) bool {
	v, ok := attrs.Get(key)
	if !ok || v.Type() != pcommon.ValueTypeStr {
		return false
	}
	parsed := pcommon.NewValueEmpty()
	if !parse(v, parsed) || parsed.Type() == pcommon.ValueTypeStr {
		return false
	}
	parsed.CopyTo(v)
	return true
}

func parseList(src, dst pcommon.Value) bool {
	var elems []string
	if json.Unmarshal([]byte(src.Str()), &elems) != nil {
		elems = []string{src.Str()}
	}
	slice := dst.SetEmptySlice()
	for _, e := range elems {
		slice.AppendEmpty().SetStr(e)
	}
	return true
}

// includeCachedInputTokens makes input_tokens include the cached tokens, as
// the gen_ai.usage.input_tokens definition requires.
func includeCachedInputTokens(attrs pcommon.Map) {
	input, ok := attrs.Get(otelsemconv.GenAIUsageInputTokens)
	if !ok || input.Type() != pcommon.ValueTypeInt {
		return
	}
	total := input.Int()
	for _, key := range []string{otelsemconv.GenAIUsageCacheReadInputTokens, otelsemconv.GenAIUsageCacheCreationInputTokens} {
		if v, ok := attrs.Get(key); ok && v.Type() == pcommon.ValueTypeInt {
			total += v.Int()
		}
	}
	input.SetInt(total)
}

func putProviderName(attrs pcommon.Map, removeOriginals, overwrite bool) bool {
	system := strAttr(attrs, genAISystem)
	if system == "" || system == frameworkSystem {
		return false
	}
	if _, exists := attrs.Get(otelsemconv.GenAIProviderName); exists && !overwrite {
		return false
	}
	provider := system
	if mapped, ok := providerNames[system]; ok {
		provider = mapped
	}
	attrs.PutStr(otelsemconv.GenAIProviderName, provider)
	if removeOriginals {
		attrs.Remove(genAISystem)
	}
	return true
}

func foldOperationName(attrs pcommon.Map) bool {
	op, ok := attrs.Get(otelsemconv.GenAIOperationName)
	if !ok || op.Type() != pcommon.ValueTypeStr {
		return false
	}
	mapped, ok := operationNames[op.Str()]
	if !ok {
		return false
	}
	op.SetStr(mapped)
	return true
}

func isStr(attrs pcommon.Map, key string) bool {
	v, ok := attrs.Get(key)
	return ok && v.Type() == pcommon.ValueTypeStr
}

func strAttr(attrs pcommon.Map, key string) string {
	v, ok := attrs.Get(key)
	if !ok || v.Type() != pcommon.ValueTypeStr {
		return ""
	}
	return v.Str()
}
