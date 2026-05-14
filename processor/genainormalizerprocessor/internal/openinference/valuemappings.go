// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// operationNameValues maps OpenInference openinference.span.kind values to
// OTel GenAI operation names. Keys are lowercased; Transform lowercases the
// input before lookup.
var operationNameValues = map[string]string{
	"llm":       "chat",
	"embedding": "embeddings",
	"chain":     "invoke_agent",
	"retriever": "retrieval",
	"reranker":  "retrieval",
	"tool":      "execute_tool",
	"agent":     "invoke_agent",
	"prompt":    "text_completion",
}

// Transformer implements the processor's valueTransformer interface for the
// OpenInference source.
type Transformer struct{}

// Transform writes a normalized representation of src into dst. For
// gen_ai.operation.name it folds the string value against operationNameValues
// (case-insensitive). All other keys and types fall through to a plain
// src.CopyTo(dst).
func (Transformer) Transform(targetKey string, src, dst pcommon.Value) {
	if targetKey == otelsemconv.GenAIOperationName && src.Type() == pcommon.ValueTypeStr {
		if mapped, ok := operationNameValues[strings.ToLower(src.Str())]; ok {
			dst.SetStr(mapped)
			return
		}
	}
	src.CopyTo(dst)
}
