// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// operationNameValues maps openinference.span.kind values to OTel GenAI
// operation names. Keys are lowercased; Transform lowercases the input
// before lookup.
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
