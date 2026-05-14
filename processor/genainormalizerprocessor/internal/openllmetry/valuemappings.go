// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openllmetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openllmetry"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// operationNameValues maps OpenLLMetry traceloop.span.kind and llm.request.type
// values to OTel GenAI operation names. The two enum spaces share one map
// because their keys do not collide today. Adding values that overlap (e.g.
// a future llm.request.type value of "tool") would require splitting into
// two maps and dispatching by source key in Transform.
//
// Keys are lowercased; Transform lowercases the input before lookup.
var operationNameValues = map[string]string{
	// traceloop.span.kind (original: lowercase)
	"workflow": "invoke_workflow",
	"task":     "invoke_agent",
	"agent":    "invoke_agent",
	"tool":     "execute_tool",

	// llm.request.type (original: lowercase)
	"completion": "text_completion",
	"chat":       "chat",
	"rerank":     "retrieval",
	"embedding":  "embeddings",
}

// Transformer implements the processor's valueTransformer interface for the
// OpenLLMetry source.
type Transformer struct{}

// Transform writes a normalized representation of src into dst:
//   - gen_ai.operation.name: fold string against operationNameValues
//     (case-insensitive)
//   - gen_ai.response.finish_reasons: wrap a string into a single-element
//     string[] to satisfy the OTel GenAI spec type
//   - gen_ai.input.messages, gen_ai.output.messages, gen_ai.tool.definitions:
//     stringify any structured source value (Map/Slice) to JSON via
//     pcommon.Value.AsString. Strings pass through. See README "Type handling"
//     for the rationale.
//
// All other keys and types fall through to a plain src.CopyTo(dst).
func (Transformer) Transform(targetKey string, src, dst pcommon.Value) {
	switch targetKey {
	case otelsemconv.GenAIOperationName:
		if src.Type() == pcommon.ValueTypeStr {
			if mapped, ok := operationNameValues[strings.ToLower(src.Str())]; ok {
				dst.SetStr(mapped)
				return
			}
		}
	case otelsemconv.GenAIResponseFinishReasons:
		if src.Type() == pcommon.ValueTypeStr {
			dst.SetEmptySlice().AppendEmpty().SetStr(src.Str())
			return
		}
	case otelsemconv.GenAIInputMessages,
		otelsemconv.GenAIOutputMessages,
		otelsemconv.GenAIToolDefinitions:
		dst.SetStr(src.AsString())
		return
	}
	src.CopyTo(dst)
}
