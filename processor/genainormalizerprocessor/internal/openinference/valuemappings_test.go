// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransformValue(t *testing.T) {
	tests := []struct {
		name      string
		targetKey string
		value     string
		want      string
	}{
		// openinference.span.kind values.
		{"span.kind LLM", "gen_ai.operation.name", "LLM", "chat"},
		{"span.kind EMBEDDING", "gen_ai.operation.name", "EMBEDDING", "embeddings"},
		{"span.kind CHAIN", "gen_ai.operation.name", "CHAIN", "invoke_agent"},
		{"span.kind RETRIEVER", "gen_ai.operation.name", "RETRIEVER", "retrieval"},
		{"span.kind RERANKER", "gen_ai.operation.name", "RERANKER", "retrieval"},
		{"span.kind TOOL", "gen_ai.operation.name", "TOOL", "execute_tool"},
		{"span.kind AGENT", "gen_ai.operation.name", "AGENT", "invoke_agent"},
		{"span.kind PROMPT", "gen_ai.operation.name", "PROMPT", "text_completion"},

		// Case-insensitivity.
		{"lowercase input", "gen_ai.operation.name", "llm", "chat"},
		{"mixed case input", "gen_ai.operation.name", "Embedding", "embeddings"},

		// Unknown values pass through unchanged.
		{"unknown value", "gen_ai.operation.name", "something-weird", "something-weird"},

		// Non-operation-name target keys pass through unchanged.
		{"unrelated target key", "gen_ai.request.model", "LLM", "LLM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Transformer{}.TransformValue(tt.targetKey, tt.value))
		})
	}
}
