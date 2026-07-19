// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openllmetry

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
		// traceloop.span.kind values.
		{"workflow", "workflow", "invoke_workflow"},
		{"task", "task", "invoke_agent"},
		{"agent", "agent", "invoke_agent"},
		{"tool", "tool", "execute_tool"},

		// llm.request.type values.
		{"completion", "completion", "text_completion"},
		{"chat", "chat", "chat"},
		{"rerank", "rerank", "retrieval"},
		{"embedding", "embedding", "embeddings"},

		// Case-insensitivity.
		{"uppercase", "WORKFLOW", "invoke_workflow"},
		{"mixed case", "Completion", "text_completion"},

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
	src := pcommon.NewValueStr("workflow")
	dst := pcommon.NewValueEmpty()
	Transform("gen_ai.request.model", src, dst)

	require.Equal(t, pcommon.ValueTypeStr, dst.Type())
	assert.Equal(t, "workflow", dst.Str())
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
