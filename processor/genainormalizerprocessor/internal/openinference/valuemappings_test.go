// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestTransform_OperationName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"LLM", "LLM", "chat"},
		{"EMBEDDING", "EMBEDDING", "embeddings"},
		{"CHAIN", "CHAIN", "invoke_agent"},
		{"RETRIEVER", "RETRIEVER", "retrieval"},
		{"RERANKER", "RERANKER", "retrieval"},
		{"TOOL", "TOOL", "execute_tool"},
		{"AGENT", "AGENT", "invoke_agent"},
		{"PROMPT", "PROMPT", "text_completion"},

		// Case-insensitivity.
		{"lowercase", "llm", "chat"},
		{"mixed case", "Embedding", "embeddings"},

		// Unknown values pass through unchanged.
		{"unknown", "something-unknown", "something-unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := pcommon.NewValueStr(tt.value)
			dst := pcommon.NewValueEmpty()
			Transform("gen_ai.operation.name", src, dst)

			require.Equal(t, pcommon.ValueTypeStr, dst.Type())
			assert.Equal(t, tt.want, dst.Str())
		})
	}
}

// TestTransform_PassthroughForUnrelatedKeys confirms the transformer is a
// no-op clone for any target key outside its known set.
func TestTransform_PassthroughForUnrelatedKeys(t *testing.T) {
	src := pcommon.NewValueStr("LLM")
	dst := pcommon.NewValueEmpty()
	Transform("gen_ai.request.model", src, dst)

	require.Equal(t, pcommon.ValueTypeStr, dst.Type())
	assert.Equal(t, "LLM", dst.Str())
}

// TestTransform_PassthroughForNonStringTypes confirms scalar int/bool/double
// copy cleanly even on keys the transformer knows about.
func TestTransform_PassthroughForNonStringTypes(t *testing.T) {
	src := pcommon.NewValueInt(42)
	dst := pcommon.NewValueEmpty()
	Transform("gen_ai.operation.name", src, dst)

	require.Equal(t, pcommon.ValueTypeInt, dst.Type())
	assert.Equal(t, int64(42), dst.Int())
}
