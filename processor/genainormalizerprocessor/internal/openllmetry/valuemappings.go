// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openllmetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openllmetry"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// operationNameValues maps OpenLLMetry traceloop.span.kind and llm.request.type
// values to OTel GenAI operation names. Keys are lowercased; Transform
// lowercases the input before lookup.
var operationNameValues = map[string]string{
	// traceloop.span.kind (original: lowercase)
	"workflow": otelsemconv.GenAIOperationNameInvokeWorkflow,
	"task":     otelsemconv.GenAIOperationNameInvokeAgent,
	"agent":    otelsemconv.GenAIOperationNameInvokeAgent,
	"tool":     otelsemconv.GenAIOperationNameExecuteTool,

	// llm.request.type (original: lowercase)
	"completion": otelsemconv.GenAIOperationNameTextCompletion,
	"chat":       otelsemconv.GenAIOperationNameChat,
	"rerank":     otelsemconv.GenAIOperationNameRetrieval,
	"embedding":  otelsemconv.GenAIOperationNameEmbeddings,
}

// Transform applies OpenLLMetry-specific value-level normalization. It folds
// llm.request.type and traceloop.span.kind strings onto the
// gen_ai.operation.name enum (case-insensitive) and copies all other values
// verbatim. Type-level enforcement (e.g. wrapping a string finish_reason
// into the spec's []string at gen_ai.response.finish_reasons) is the
// processor's responsibility via otelsemconv.Coerce.
func Transform(targetKey string, src, dst pcommon.Value) {
	if targetKey == otelsemconv.GenAIOperationName && src.Type() == pcommon.ValueTypeStr {
		if mapped, ok := operationNameValues[strings.ToLower(src.Str())]; ok {
			dst.SetStr(mapped)
			return
		}
	}
	src.CopyTo(dst)
}
