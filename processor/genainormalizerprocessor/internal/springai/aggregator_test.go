// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package springai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestAggregate_RetypesStringTokenCounts(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.usage.input_tokens", "758")
	attrs.PutStr("gen_ai.usage.output_tokens", "43")
	attrs.PutStr("gen_ai.usage.cache_creation.input_tokens", "5")
	attrs.PutStr("gen_ai.usage.cache_read.input_tokens", "7")
	attrs.PutStr("gen_ai.request.max_tokens", "4096")

	assert.True(t, aggregate(attrs, false))

	for key, want := range map[string]int64{
		"gen_ai.usage.input_tokens":                758,
		"gen_ai.usage.output_tokens":               43,
		"gen_ai.usage.cache_creation.input_tokens": 5,
		"gen_ai.usage.cache_read.input_tokens":     7,
		"gen_ai.request.max_tokens":                4096,
	} {
		v, ok := attrs.Get(key)
		require.True(t, ok, key)
		assert.Equal(t, pcommon.ValueTypeInt, v.Type(), key)
		assert.Equal(t, want, v.Int(), key)
	}
}

func TestAggregate_RetypesStringFloats(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.request.temperature", "0.7")
	attrs.PutStr("gen_ai.request.top_p", "0.9")
	attrs.PutStr("gen_ai.request.top_k", "40")
	attrs.PutStr("gen_ai.request.frequency_penalty", "0.5")
	attrs.PutStr("gen_ai.request.presence_penalty", "-0.5")

	aggregate(attrs, false)

	for key, want := range map[string]float64{
		"gen_ai.request.temperature":       0.7,
		"gen_ai.request.top_p":             0.9,
		"gen_ai.request.top_k":             40,
		"gen_ai.request.frequency_penalty": 0.5,
		"gen_ai.request.presence_penalty":  -0.5,
	} {
		v, ok := attrs.Get(key)
		require.True(t, ok, key)
		assert.Equal(t, pcommon.ValueTypeDouble, v.Type(), key)
		assert.InDelta(t, want, v.Double(), 1e-9, key)
	}
}

func TestAggregate_LeavesUnparsableNumbersAlone(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.usage.input_tokens", "n/a")

	assert.False(t, aggregate(attrs, false))

	v, _ := attrs.Get("gen_ai.usage.input_tokens")
	assert.Equal(t, pcommon.ValueTypeStr, v.Type())
	assert.Equal(t, "n/a", v.Str())
}

func TestAggregate_KeepsAlreadyTypedValues(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutInt("gen_ai.usage.input_tokens", 12)

	assert.False(t, aggregate(attrs, false))

	v, _ := attrs.Get("gen_ai.usage.input_tokens")
	assert.Equal(t, int64(12), v.Int())
}

// concatenateStrings is Spring AI's ObservabilityHelper.concatenateStrings.
func concatenateStrings(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = `"` + item + `"`
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func TestAggregate_DecodesSpringListsWithoutChangingTheElements(t *testing.T) {
	tests := map[string][]string{
		"finish reasons":      {"tool_use", "tool_use"},
		"real newlines":       {"\n\n", "END"},
		"literal backslash-n": {`\n`},
		"quotes":              {`say "hi"`},
		"brackets":            {"[x]"},
		"one empty element":   {""},
		"no elements":         {},
	}
	for name, items := range tests {
		t.Run(name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			attrs.PutStr("gen_ai.request.stop_sequences", concatenateStrings(items))

			aggregate(attrs, false)

			v, _ := attrs.Get("gen_ai.request.stop_sequences")
			require.Equal(t, pcommon.ValueTypeSlice, v.Type())
			got := make([]string, 0, v.Slice().Len())
			for i := range v.Slice().Len() {
				got = append(got, v.Slice().At(i).Str())
			}
			assert.Equal(t, items, got)
		})
	}
}

func TestAggregate_WrapsAValueThatIsNotASpringList(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.response.finish_reasons", "stop")

	aggregate(attrs, false)

	v, _ := attrs.Get("gen_ai.response.finish_reasons")
	require.Equal(t, pcommon.ValueTypeSlice, v.Type())
	assert.Equal(t, []any{"stop"}, v.Slice().AsRaw())
}

func TestAggregate_RetypesStream(t *testing.T) {
	for in, want := range map[string]bool{"true": true, "false": false} {
		attrs := pcommon.NewMap()
		attrs.PutStr("gen_ai.request.stream", in)

		assert.True(t, aggregate(attrs, false))

		v, _ := attrs.Get("gen_ai.request.stream")
		require.Equal(t, pcommon.ValueTypeBool, v.Type(), in)
		assert.Equal(t, want, v.Bool(), in)
	}
}

func TestAggregate_LeavesUnparsableStreamAlone(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.request.stream", "sometimes")

	assert.False(t, aggregate(attrs, false))

	v, _ := attrs.Get("gen_ai.request.stream")
	assert.Equal(t, "sometimes", v.Str())
}

func TestAggregate_ProviderName(t *testing.T) {
	tests := []struct {
		system   string
		provider string
	}{
		{"anthropic", "anthropic"},
		{"openai", "openai"},
		{"mistral_ai", "mistral_ai"},
		{"deepseek", "deepseek"},
		{"bedrock_converse", "aws.bedrock"},
		{"vertex_ai", "gcp.vertex_ai"},
		{"google_genai", "gcp.gen_ai"},
		{"ollama", "ollama"},
	}
	for _, tt := range tests {
		t.Run(tt.system, func(t *testing.T) {
			attrs := pcommon.NewMap()
			attrs.PutStr("gen_ai.system", tt.system)

			assert.True(t, aggregate(attrs, false))

			v, ok := attrs.Get("gen_ai.provider.name")
			require.True(t, ok)
			assert.Equal(t, tt.provider, v.Str())
		})
	}
}

func TestAggregate_SpringAIIsNotAProvider(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.system", "spring_ai")

	assert.False(t, aggregate(attrs, false))

	_, ok := attrs.Get("gen_ai.provider.name")
	assert.False(t, ok)
}

func TestAggregate_ProviderNameRespectsOverwrite(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.system", "openai")
	attrs.PutStr("gen_ai.provider.name", "azure.ai.openai")

	aggregate(attrs, false)
	v, _ := attrs.Get("gen_ai.provider.name")
	assert.Equal(t, "azure.ai.openai", v.Str())

	aggregate(attrs, true)
	v, _ = attrs.Get("gen_ai.provider.name")
	assert.Equal(t, "openai", v.Str())
}

func TestAggregate_OperationName(t *testing.T) {
	tests := []struct {
		name string
		op   string
		want string
	}{
		{"embedding folds onto the plural enum value", "embedding", "embeddings"},
		{"chat is already canonical", "chat", "chat"},
		{"execute_tool is already canonical", "execute_tool", "execute_tool"},
		{"framework has no OTel counterpart", "framework", "framework"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			attrs.PutStr("gen_ai.operation.name", tt.op)

			aggregate(attrs, false)

			v, _ := attrs.Get("gen_ai.operation.name")
			assert.Equal(t, tt.want, v.Str())
		})
	}
}

// Spring AI's Anthropic model reports Anthropic's input_tokens, which
// excludes cached tokens, while the semconv count includes them.
func TestAggregate_AnthropicInputTokensIncludeCache(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.system", "anthropic")
	attrs.PutStr("gen_ai.usage.input_tokens", "1000")
	attrs.PutStr("gen_ai.usage.cache_read.input_tokens", "200")
	attrs.PutStr("gen_ai.usage.cache_creation.input_tokens", "100")

	aggregate(attrs, false)

	v, _ := attrs.Get("gen_ai.usage.input_tokens")
	assert.Equal(t, int64(1300), v.Int())
}

func TestAggregate_AnthropicInputTokensAreFoldedOnce(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.system", "anthropic")
	attrs.PutStr("gen_ai.usage.input_tokens", "1000")
	attrs.PutStr("gen_ai.usage.cache_read.input_tokens", "200")

	aggregate(attrs, false)
	aggregate(attrs, false)

	v, _ := attrs.Get("gen_ai.usage.input_tokens")
	assert.Equal(t, int64(1200), v.Int())
}

func TestAggregate_TypedAnthropicInputTokensAreLeftAlone(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.system", "anthropic")
	attrs.PutInt("gen_ai.usage.input_tokens", 1000)
	attrs.PutInt("gen_ai.usage.cache_read.input_tokens", 200)

	aggregate(attrs, false)

	v, _ := attrs.Get("gen_ai.usage.input_tokens")
	assert.Equal(t, int64(1000), v.Int())
}

func TestAggregate_OtherProvidersAlreadyIncludeCache(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.system", "openai")
	attrs.PutStr("gen_ai.usage.input_tokens", "1000")
	attrs.PutStr("gen_ai.usage.cache_read.input_tokens", "200")

	aggregate(attrs, false)

	v, _ := attrs.Get("gen_ai.usage.input_tokens")
	assert.Equal(t, int64(1000), v.Int())
}

func TestAggregate_RemoveOriginalsDropsSystem(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.system", "openai")

	Aggregator{}.AggregateAttributes(attrs, true, false)

	_, ok := attrs.Get("gen_ai.system")
	assert.False(t, ok)
	v, _ := attrs.Get("gen_ai.provider.name")
	assert.Equal(t, "openai", v.Str())
}

func TestAggregate_RemoveOriginalsKeepsFrameworkSystem(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("gen_ai.system", "spring_ai")

	Aggregator{}.AggregateAttributes(attrs, true, false)

	v, ok := attrs.Get("gen_ai.system")
	require.True(t, ok)
	assert.Equal(t, "spring_ai", v.Str())
}

func aggregate(attrs pcommon.Map, overwrite bool) bool {
	return Aggregator{}.AggregateAttributes(attrs, false, overwrite)
}
