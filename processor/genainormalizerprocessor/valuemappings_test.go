package genainormalizerprocessor

import "testing"

func TestOpenInferenceSpanKindMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"LLM", "chat"},
		{"EMBEDDING", "embeddings"},
		{"TOOL", "execute_tool"},
		{"AGENT", "invoke_agent"},
		{"CHAIN", "invoke_agent"},
		{"RETRIEVER", "retrieval"},
		{"RERANKER", "retrieval"},
		{"PROMPT", "text_completion"},
	}
	for _, tt := range tests {
		got := TransformValue("gen_ai.operation.name", tt.input)
		if got != tt.expected {
			t.Errorf("TransformValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestOpenLLMetrySpanKindMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"workflow", "invoke_agent"},
		{"task", "invoke_agent"},
		{"agent", "invoke_agent"},
		{"tool", "execute_tool"},
	}
	for _, tt := range tests {
		got := TransformValue("gen_ai.operation.name", tt.input)
		if got != tt.expected {
			t.Errorf("TransformValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestOpenLLMetryRequestTypeMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"completion", "text_completion"},
		{"chat", "chat"},
		{"rerank", "retrieval"},
		{"embedding", "embeddings"},
	}
	for _, tt := range tests {
		got := TransformValue("gen_ai.operation.name", tt.input)
		if got != tt.expected {
			t.Errorf("TransformValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestUnknownValuePassthrough(t *testing.T) {
	got := TransformValue("gen_ai.operation.name", "some_custom_value")
	if got != "some_custom_value" {
		t.Errorf("expected passthrough, got %q", got)
	}
}

func TestNonMappedKeyPassthrough(t *testing.T) {
	got := TransformValue("gen_ai.request.model", "gpt-4")
	if got != "gpt-4" {
		t.Errorf("expected passthrough, got %q", got)
	}
}
