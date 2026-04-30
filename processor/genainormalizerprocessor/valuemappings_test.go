// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransformValue(t *testing.T) {
	tests := []struct {
		name      string
		targetKey string
		input     string
		want      string
	}{
		// OpenInference openinference.span.kind
		{"openinference/LLM", targetOperationName, "LLM", "chat"},
		{"openinference/EMBEDDING", targetOperationName, "EMBEDDING", "embeddings"},
		{"openinference/TOOL", targetOperationName, "TOOL", "execute_tool"},
		{"openinference/AGENT", targetOperationName, "AGENT", "invoke_agent"},
		{"openinference/CHAIN", targetOperationName, "CHAIN", "invoke_agent"},
		{"openinference/RETRIEVER", targetOperationName, "RETRIEVER", "retrieval"},
		{"openinference/RERANKER", targetOperationName, "RERANKER", "retrieval"},
		{"openinference/PROMPT", targetOperationName, "PROMPT", "text_completion"},

		// OpenLLMetry traceloop.span.kind
		{"openllmetry/workflow", targetOperationName, "workflow", "invoke_workflow"},
		{"openllmetry/task", targetOperationName, "task", "invoke_agent"},
		{"openllmetry/agent", targetOperationName, "agent", "invoke_agent"},
		{"openllmetry/tool", targetOperationName, "tool", "execute_tool"},

		// OpenLLMetry llm.request.type
		{"openllmetry/completion", targetOperationName, "completion", "text_completion"},
		{"openllmetry/chat", targetOperationName, "chat", "chat"},
		{"openllmetry/rerank", targetOperationName, "rerank", "retrieval"},
		{"openllmetry/embedding", targetOperationName, "embedding", "embeddings"},

		// Case-insensitivity
		{"case_insensitive/llm", targetOperationName, "llm", "chat"},
		{"case_insensitive/LLm", targetOperationName, "LLm", "chat"},

		// Unknown value passes through
		{"unknown_value_passthrough", targetOperationName, "some_custom_value", "some_custom_value"},

		// Unrelated target key passes through
		{"non_mapped_key_passthrough", "gen_ai.request.model", "gpt-4", "gpt-4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, transformValue(tt.targetKey, tt.input))
		})
	}
}
