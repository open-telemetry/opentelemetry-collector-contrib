// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

func TestNormalizeGenaiAttributes_AllProfiles(t *testing.T) {
	span := ptrace.NewSpan()
	span.Attributes().PutInt("llm.token_count.prompt", 100)
	span.Attributes().PutStr("llm.model_name", "gpt-4")
	span.Attributes().PutStr("openinference.span.kind", "LLM")
	span.Attributes().PutStr("agent.name", "researcher")

	lookupTable := buildGenaiLookupTable(allProfileNames)
	normalizeGenaiAttributes(span, lookupTable, false)

	val, ok := span.Attributes().Get("gen_ai.usage.input_tokens")
	require.True(t, ok)
	assert.Equal(t, int64(100), val.Int())

	val, ok = span.Attributes().Get("gen_ai.request.model")
	require.True(t, ok)
	assert.Equal(t, "gpt-4", val.Str())

	val, ok = span.Attributes().Get("gen_ai.operation.name")
	require.True(t, ok)
	assert.Equal(t, "chat", val.Str())

	val, ok = span.Attributes().Get("gen_ai.agent.name")
	require.True(t, ok)
	assert.Equal(t, "researcher", val.Str())

	// Originals should still exist (remove_originals=false)
	_, ok = span.Attributes().Get("llm.token_count.prompt")
	assert.True(t, ok)
}

func TestNormalizeGenaiAttributes_RemoveOriginals(t *testing.T) {
	span := ptrace.NewSpan()
	span.Attributes().PutInt("llm.usage.prompt_tokens", 200)
	span.Attributes().PutStr("llm.request.model", "claude-3")

	lookupTable := buildGenaiLookupTable([]string{"openllmetry"})
	normalizeGenaiAttributes(span, lookupTable, true)

	_, ok := span.Attributes().Get("gen_ai.usage.input_tokens")
	assert.True(t, ok)

	// Originals should be removed
	_, ok = span.Attributes().Get("llm.usage.prompt_tokens")
	assert.False(t, ok)
	_, ok = span.Attributes().Get("llm.request.model")
	assert.False(t, ok)
}

func TestNormalizeGenaiAttributes_ValueMapping(t *testing.T) {
	tests := []struct {
		name     string
		srcKey   string
		srcVal   string
		wantKey  string
		wantVal  string
		profiles []string
	}{
		{"OpenInference LLM", "openinference.span.kind", "LLM", "gen_ai.operation.name", "chat", []string{"openinference"}},
		{"OpenInference AGENT", "openinference.span.kind", "AGENT", "gen_ai.operation.name", "invoke_agent", []string{"openinference"}},
		{"OpenInference TOOL", "openinference.span.kind", "TOOL", "gen_ai.operation.name", "execute_tool", []string{"openinference"}},
		{"OpenLLMetry chat", "llm.request.type", "chat", "gen_ai.operation.name", "chat", []string{"openllmetry"}},
		{"OpenLLMetry workflow", "traceloop.span.kind", "workflow", "gen_ai.operation.name", "invoke_agent", []string{"openllmetry"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.Attributes().PutStr(tt.srcKey, tt.srcVal)

			lookupTable := buildGenaiLookupTable(tt.profiles)
			normalizeGenaiAttributes(span, lookupTable, true)

			val, ok := span.Attributes().Get(tt.wantKey)
			require.True(t, ok)
			assert.Equal(t, tt.wantVal, val.Str())
		})
	}
}

func TestNormalizeGenaiAttributes_SpecificProfiles(t *testing.T) {
	span := ptrace.NewSpan()
	span.Attributes().PutInt("llm.token_count.prompt", 100)  // openinference
	span.Attributes().PutInt("llm.usage.prompt_tokens", 200) // openllmetry

	// Only enable openinference
	lookupTable := buildGenaiLookupTable([]string{"openinference"})
	normalizeGenaiAttributes(span, lookupTable, true)

	// openinference mapping should apply
	val, ok := span.Attributes().Get("gen_ai.usage.input_tokens")
	require.True(t, ok)
	assert.Equal(t, int64(100), val.Int())

	// openllmetry source should be untouched
	_, ok = span.Attributes().Get("llm.usage.prompt_tokens")
	assert.True(t, ok)
}

func TestNormalizeGenaiAttributes_WrapSlice(t *testing.T) {
	span := ptrace.NewSpan()
	span.Attributes().PutStr("llm.response.finish_reason", "stop")

	lookupTable := buildGenaiLookupTable([]string{"openllmetry"})
	normalizeGenaiAttributes(span, lookupTable, true)

	val, ok := span.Attributes().Get("gen_ai.response.finish_reasons")
	require.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeSlice, val.Type())
	assert.Equal(t, 1, val.Slice().Len())
	assert.Equal(t, "stop", val.Slice().At(0).Str())
}

func TestNormalizeGenaiAttributes_NoGenaiSpan(t *testing.T) {
	span := ptrace.NewSpan()
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutStr("http.url", "https://example.com")

	lookupTable := buildGenaiLookupTable(allProfileNames)
	normalizeGenaiAttributes(span, lookupTable, true)

	// Nothing should change
	assert.Equal(t, 2, span.Attributes().Len())
}

func TestCreateNormalizeGenaiAttributesFunction_UnsupportedVersion(t *testing.T) {
	_, err := createNormalizeGenaiAttributesFunction(ottl.FunctionContext{}, &normalizeGenaiAttributesArguments{
		SemconvVersion: "1.0.0",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported semconv version")
}

func TestCreateNormalizeGenaiAttributesFunction_UnknownProfile(t *testing.T) {
	_, err := createNormalizeGenaiAttributesFunction(ottl.FunctionContext{}, &normalizeGenaiAttributesArguments{
		SemconvVersion: supportedGenAISemconvVersion,
		Profiles:       ottl.NewTestingOptional([]string{"nonexistent"}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown profile")
}

func TestCreateNormalizeGenaiAttributesFunction_Integration(t *testing.T) {
	fn, err := createNormalizeGenaiAttributesFunction(ottl.FunctionContext{}, &normalizeGenaiAttributesArguments{
		SemconvVersion: supportedGenAISemconvVersion,
	})
	require.NoError(t, err)

	span := ptrace.NewSpan()
	span.Attributes().PutInt("llm.token_count.prompt", 50)
	span.Attributes().PutStr("openinference.span.kind", "AGENT")

	tCtx := ottlspan.NewTransformContextPtr(ptrace.NewResourceSpans(), ptrace.NewScopeSpans(), span)
	_, err = fn(context.Background(), tCtx)
	require.NoError(t, err)

	val, ok := span.Attributes().Get("gen_ai.usage.input_tokens")
	require.True(t, ok)
	assert.Equal(t, int64(50), val.Int())

	val, ok = span.Attributes().Get("gen_ai.operation.name")
	require.True(t, ok)
	assert.Equal(t, "invoke_agent", val.Str())
}
