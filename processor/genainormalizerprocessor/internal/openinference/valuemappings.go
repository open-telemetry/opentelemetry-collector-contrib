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
// constants (lowercased so Transform can do a case-insensitive lookup);
// target values come from otelsemconv's GenAIOperationName* registry.
var operationNameValues = map[string]string{
	strings.ToLower(oisemconv.SpanKindLLM):       otelsemconv.GenAIOperationNameChat,
	strings.ToLower(oisemconv.SpanKindEmbedding): otelsemconv.GenAIOperationNameEmbeddings,
	strings.ToLower(oisemconv.SpanKindChain):     otelsemconv.GenAIOperationNameInvokeAgent,
	strings.ToLower(oisemconv.SpanKindRetriever): otelsemconv.GenAIOperationNameRetrieval,
	strings.ToLower(oisemconv.SpanKindReranker):  otelsemconv.GenAIOperationNameRetrieval,
	strings.ToLower(oisemconv.SpanKindTool):      otelsemconv.GenAIOperationNameExecuteTool,
	strings.ToLower(oisemconv.SpanKindAgent):     otelsemconv.GenAIOperationNameInvokeAgent,
	strings.ToLower(oisemconv.SpanKindPrompt):    otelsemconv.GenAIOperationNameTextCompletion,
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
