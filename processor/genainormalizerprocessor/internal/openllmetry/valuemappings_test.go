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
			Transformer{}.Transform("gen_ai.operation.name", src, dst)

			require.Equal(t, pcommon.ValueTypeStr, dst.Type())
			assert.Equal(t, tt.want, dst.Str())
		})
	}
}

// TestTransform_FinishReasonWrapsToSlice confirms a string source value is
// wrapped into a single-element string[] when written to
// gen_ai.response.finish_reasons. The OTel GenAI spec defines that key as
// string[]; sources like OpenLLMetry emit a single string.
func TestTransform_FinishReasonWrapsToSlice(t *testing.T) {
	src := pcommon.NewValueStr("stop")
	dst := pcommon.NewValueEmpty()
	Transformer{}.Transform("gen_ai.response.finish_reasons", src, dst)

	require.Equal(t, pcommon.ValueTypeSlice, dst.Type())
	require.Equal(t, 1, dst.Slice().Len())
	assert.Equal(t, "stop", dst.Slice().At(0).Str())
}

// TestTransform_PassthroughForUnrelatedKeys confirms the transformer is a
// no-op clone for any target key outside its known set.
func TestTransform_PassthroughForUnrelatedKeys(t *testing.T) {
	src := pcommon.NewValueStr("workflow")
	dst := pcommon.NewValueEmpty()
	Transformer{}.Transform("gen_ai.request.model", src, dst)

	require.Equal(t, pcommon.ValueTypeStr, dst.Type())
	assert.Equal(t, "workflow", dst.Str())
}

// TestTransform_PassthroughForNonStringTypes confirms scalar int/bool/double
// copy cleanly even on keys the transformer knows about.
func TestTransform_PassthroughForNonStringTypes(t *testing.T) {
	src := pcommon.NewValueInt(42)
	dst := pcommon.NewValueEmpty()
	Transformer{}.Transform("gen_ai.operation.name", src, dst)

	require.Equal(t, pcommon.ValueTypeInt, dst.Type())
	assert.Equal(t, int64(42), dst.Int())
}

// TestTransform_StructuredTargetsStringify confirms that the three "string
// everywhere" target keys (gen_ai.input.messages, gen_ai.output.messages,
// gen_ai.tool.definitions) always land as strings:
//   - string source: pass through verbatim
//   - structured source (slice of maps): JSON-encoded via pcommon.AsString
func TestTransform_StructuredTargetsStringify(t *testing.T) {
	stringTargets := []string{
		"gen_ai.input.messages",
		"gen_ai.output.messages",
		"gen_ai.tool.definitions",
	}

	for _, key := range stringTargets {
		t.Run("string passthrough "+key, func(t *testing.T) {
			src := pcommon.NewValueStr(`[{"role":"user","content":"hi"}]`)
			dst := pcommon.NewValueEmpty()
			Transformer{}.Transform(key, src, dst)

			require.Equal(t, pcommon.ValueTypeStr, dst.Type())
			assert.Equal(t, `[{"role":"user","content":"hi"}]`, dst.Str())
		})

		t.Run("structured slice stringified "+key, func(t *testing.T) {
			src := pcommon.NewValueSlice()
			m := src.Slice().AppendEmpty().SetEmptyMap()
			m.PutStr("role", "user")
			m.PutStr("content", "hi")

			dst := pcommon.NewValueEmpty()
			Transformer{}.Transform(key, src, dst)

			require.Equal(t, pcommon.ValueTypeStr, dst.Type())
			assert.Equal(t, `[{"content":"hi","role":"user"}]`, dst.Str())
		})
	}
}
