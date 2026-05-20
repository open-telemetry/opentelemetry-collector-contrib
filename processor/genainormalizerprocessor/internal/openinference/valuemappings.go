// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	"strings"

	oisemconv "github.com/Arize-ai/openinference/go/openinference-semantic-conventions"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// operationNameValues maps openinference.span.kind values to OTel GenAI
// operation names. Source-side keys come from the upstream SpanKind*
// constants, lowercased so Transform can do a case-insensitive lookup.
var operationNameValues = map[string]string{
	strings.ToLower(oisemconv.SpanKindLLM):       "chat",
	strings.ToLower(oisemconv.SpanKindEmbedding): "embeddings",
	strings.ToLower(oisemconv.SpanKindChain):     "invoke_agent",
	strings.ToLower(oisemconv.SpanKindRetriever): "retrieval",
	strings.ToLower(oisemconv.SpanKindReranker):  "retrieval",
	strings.ToLower(oisemconv.SpanKindTool):      "execute_tool",
	strings.ToLower(oisemconv.SpanKindAgent):     "invoke_agent",
	strings.ToLower(oisemconv.SpanKindPrompt):    "text_completion",
}

// Transform applies OpenInference-specific value-level normalization. It
// folds openinference.span.kind strings onto the gen_ai.operation.name enum
// (case-insensitive) and copies all other values verbatim. Type-level
// enforcement is the processor's responsibility via otelsemconv.Coerce.
func Transform(targetKey string, src, dst pcommon.Value) {
	if targetKey == otelsemconv.GenAIOperationName && src.Type() == pcommon.ValueTypeStr {
		if mapped, ok := operationNameValues[strings.ToLower(src.Str())]; ok {
			dst.SetStr(mapped)
			return
		}
	}
	src.CopyTo(dst)
}
