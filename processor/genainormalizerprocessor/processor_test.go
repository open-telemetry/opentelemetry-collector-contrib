// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"
)

// newNormalizer builds a single-source sourceNormalizer from a fixture lookup
// table so machinery tests do not depend on the populated built-in tables.
func newNormalizer(lookup map[string]string, removeOriginals, overwrite bool) sourceNormalizer {
	return sourceNormalizer{
		lookupTable:     lookup,
		removeOriginals: removeOriginals,
		overwrite:       overwrite,
	}
}

func newSpan() (ptrace.Traces, ptrace.Span) {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	return td, span
}

func TestNormalizeAttributes(t *testing.T) {
	tests := []struct {
		name            string
		lookup          map[string]string
		transformValue  valueTransformer
		removeOriginals bool
		overwrite       bool
		setup           func(pcommon.Map)
		verify          func(*testing.T, pcommon.Map)
	}{
		{
			name:            "string rename, remove_originals=true",
			lookup:          map[string]string{"src.model": "dst.model"},
			removeOriginals: true,
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("src.model", "m")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("dst.model")
				require.True(t, ok)
				assert.Equal(t, "m", v.Str())
				_, ok = attrs.Get("src.model")
				assert.False(t, ok)
			},
		},
		{
			name:   "int rename",
			lookup: map[string]string{"src.count": "dst.count"},
			setup: func(attrs pcommon.Map) {
				attrs.PutInt("src.count", 42)
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("dst.count")
				require.True(t, ok)
				assert.Equal(t, int64(42), v.Int())
			},
		},
		{
			name:   "double rename",
			lookup: map[string]string{"src.ratio": "dst.ratio"},
			setup: func(attrs pcommon.Map) {
				attrs.PutDouble("src.ratio", 0.7)
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("dst.ratio")
				require.True(t, ok)
				assert.Equal(t, 0.7, v.Double())
			},
		},
		{
			name:   "bool rename",
			lookup: map[string]string{"src.flag": "dst.flag"},
			setup: func(attrs pcommon.Map) {
				attrs.PutBool("src.flag", true)
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("dst.flag")
				require.True(t, ok)
				assert.True(t, v.Bool())
			},
		},
		{
			name:   "slice rename snapshots source before write",
			lookup: map[string]string{"src.list": "dst.list"},
			setup: func(attrs pcommon.Map) {
				s := attrs.PutEmptySlice("src.list")
				s.AppendEmpty().SetStr("a")
				s.AppendEmpty().SetStr("b")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("dst.list")
				require.True(t, ok)
				require.Equal(t, pcommon.ValueTypeSlice, v.Type())
				require.Equal(t, 2, v.Slice().Len())
				assert.Equal(t, "a", v.Slice().At(0).Str())
				assert.Equal(t, "b", v.Slice().At(1).Str())
			},
		},
		{
			name:   "map rename snapshots source before write",
			lookup: map[string]string{"src.obj": "dst.obj"},
			setup: func(attrs pcommon.Map) {
				m := attrs.PutEmptyMap("src.obj")
				m.PutStr("k", "v")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("dst.obj")
				require.True(t, ok)
				require.Equal(t, pcommon.ValueTypeMap, v.Type())
				inner, ok := v.Map().Get("k")
				require.True(t, ok)
				assert.Equal(t, "v", inner.Str())
			},
		},
		{
			name:   "bytes rename snapshots source before write",
			lookup: map[string]string{"src.bytes": "dst.bytes"},
			setup: func(attrs pcommon.Map) {
				attrs.PutEmptyBytes("src.bytes").FromRaw([]byte{0x01, 0x02})
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("dst.bytes")
				require.True(t, ok)
				require.Equal(t, pcommon.ValueTypeBytes, v.Type())
				assert.Equal(t, []byte{0x01, 0x02}, v.Bytes().AsRaw())
			},
		},
		{
			name:            "remove_originals=false keeps source",
			lookup:          map[string]string{"src.model": "dst.model"},
			removeOriginals: false,
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("src.model", "m")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				_, ok := attrs.Get("src.model")
				assert.True(t, ok)
			},
		},
		{
			name:            "overwrite=false skips when target exists",
			lookup:          map[string]string{"src.model": "dst.model"},
			removeOriginals: true,
			overwrite:       false,
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("src.model", "new")
				attrs.PutStr("dst.model", "existing")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, _ := attrs.Get("dst.model")
				assert.Equal(t, "existing", v.Str())
			},
		},
		{
			name:            "overwrite=true replaces when target exists",
			lookup:          map[string]string{"src.model": "dst.model"},
			removeOriginals: true,
			overwrite:       true,
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("src.model", "new")
				attrs.PutStr("dst.model", "existing")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, _ := attrs.Get("dst.model")
				assert.Equal(t, "new", v.Str())
			},
		},
		{
			name:            "no match is a no-op",
			lookup:          map[string]string{"src.model": "dst.model"},
			removeOriginals: true,
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
				attrs.PutInt("http.status_code", 200)
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assert.Equal(t, 2, attrs.Len())
			},
		},
		{
			name: "multiple renames in one pass",
			lookup: map[string]string{
				"src.a": "dst.a",
				"src.b": "dst.b",
			},
			removeOriginals: true,
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("src.a", "va")
				attrs.PutStr("src.b", "vb")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				va, _ := attrs.Get("dst.a")
				vb, _ := attrs.Get("dst.b")
				assert.Equal(t, "va", va.Str())
				assert.Equal(t, "vb", vb.Str())
			},
		},
		{
			name:           "string target routed through transformValue",
			lookup:         map[string]string{"src.op": "gen_ai.operation.name"},
			transformValue: openinference.Transformer{},
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("src.op", "LLM")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("gen_ai.operation.name")
				require.True(t, ok)
				assert.Equal(t, "chat", v.Str())
			},
		},
		{
			name:   "nil transformValue passes string through unchanged",
			lookup: map[string]string{"src.op": "gen_ai.operation.name"},
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("src.op", "LLM")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("gen_ai.operation.name")
				require.True(t, ok)
				assert.Equal(t, "LLM", v.Str())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, span := newSpan()
			tt.setup(span.Attributes())
			sn := newNormalizer(tt.lookup, tt.removeOriginals, tt.overwrite)
			sn.transformValue = tt.transformValue
			sn.normalizeAttributes(span.Attributes())
			tt.verify(t, span.Attributes())
		})
	}
}

func TestNormalizeAttributes_ReportsWrote(t *testing.T) {
	lookup := map[string]string{"src.model": "dst.model"}

	t.Run("returns true on write", func(t *testing.T) {
		_, span := newSpan()
		span.Attributes().PutStr("src.model", "m")
		sn := newNormalizer(lookup, true, false)
		assert.True(t, sn.normalizeAttributes(span.Attributes()))
	})

	t.Run("returns false when no match", func(t *testing.T) {
		_, span := newSpan()
		span.Attributes().PutStr("http.method", "GET")
		sn := newNormalizer(lookup, true, false)
		assert.False(t, sn.normalizeAttributes(span.Attributes()))
	})

	t.Run("returns false when overwrite=false and target exists", func(t *testing.T) {
		_, span := newSpan()
		span.Attributes().PutStr("src.model", "new")
		span.Attributes().PutStr("dst.model", "existing")
		sn := newNormalizer(lookup, true, false)
		assert.False(t, sn.normalizeAttributes(span.Attributes()))
	})
}

func TestProcessTraces_AppliesToSpans(t *testing.T) {
	p := &genaiNormalizerProcessor{
		sources: []sourceNormalizer{
			newNormalizer(map[string]string{"src.model": "dst.model"}, true, false),
		},
	}

	td, span := newSpan()
	span.Attributes().PutStr("src.model", "m")

	_, err := p.processTraces(t.Context(), td)
	require.NoError(t, err)

	v, ok := span.Attributes().Get("dst.model")
	require.True(t, ok)
	assert.Equal(t, "m", v.Str())
}

func TestProcessTraces_StampsSchemaURLWhenMappingFires(t *testing.T) {
	p := &genaiNormalizerProcessor{
		sources: []sourceNormalizer{
			newNormalizer(map[string]string{"src.model": "dst.model"}, true, false),
		},
	}

	td, span := newSpan()
	span.Attributes().PutStr("src.model", "m")

	_, err := p.processTraces(t.Context(), td)
	require.NoError(t, err)

	ss := td.ResourceSpans().At(0).ScopeSpans().At(0)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.40.0", ss.SchemaUrl())
}

func TestProcessTraces_LeavesSchemaURLWhenNoMappingFires(t *testing.T) {
	p := &genaiNormalizerProcessor{
		sources: []sourceNormalizer{
			newNormalizer(map[string]string{"src.model": "dst.model"}, true, false),
		},
	}

	td, span := newSpan()
	span.Attributes().PutStr("http.method", "GET")

	_, err := p.processTraces(t.Context(), td)
	require.NoError(t, err)

	ss := td.ResourceSpans().At(0).ScopeSpans().At(0)
	assert.Empty(t, ss.SchemaUrl(), "schema_url must not be set when no mapping fires")
}

func TestProcessTraces_PreservesExistingSchemaURL(t *testing.T) {
	// Existing schema_url must not be overridden. Other attributes on the
	// scope may still follow the original semantic conventions, and
	// normalizing only the GenAI attributes while rewriting the schema_url
	// would misrepresent those.
	p := &genaiNormalizerProcessor{
		sources: []sourceNormalizer{
			newNormalizer(map[string]string{"src.model": "dst.model"}, true, false),
		},
	}

	td, span := newSpan()
	td.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("https://opentelemetry.io/schemas/1.38.0")
	span.Attributes().PutStr("src.model", "m")

	_, err := p.processTraces(t.Context(), td)
	require.NoError(t, err)

	ss := td.ResourceSpans().At(0).ScopeSpans().At(0)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.38.0", ss.SchemaUrl())
}

func TestProcessTraces_DoesNotModifyResourceSchemaURL(t *testing.T) {
	p := &genaiNormalizerProcessor{
		sources: []sourceNormalizer{
			newNormalizer(map[string]string{"src.model": "dst.model"}, true, false),
		},
	}

	td, span := newSpan()
	td.ResourceSpans().At(0).SetSchemaUrl("https://opentelemetry.io/schemas/1.38.0")
	span.Attributes().PutStr("src.model", "m")

	_, err := p.processTraces(t.Context(), td)
	require.NoError(t, err)

	assert.Equal(t,
		"https://opentelemetry.io/schemas/1.38.0",
		td.ResourceSpans().At(0).SchemaUrl(),
		"resource schema_url must not be modified",
	)
}

func TestProcessTraces_IgnoresSpanEvents(t *testing.T) {
	p := &genaiNormalizerProcessor{
		sources: []sourceNormalizer{
			newNormalizer(map[string]string{"src.model": "dst.model"}, true, false),
		},
	}

	td, span := newSpan()
	evt := span.Events().AppendEmpty()
	evt.Attributes().PutStr("src.model", "m")

	_, err := p.processTraces(t.Context(), td)
	require.NoError(t, err)

	evtAttrs := span.Events().At(0).Attributes()
	_, ok := evtAttrs.Get("dst.model")
	assert.False(t, ok, "span event attributes must not be modified")
	v, ok := evtAttrs.Get("src.model")
	require.True(t, ok)
	assert.Equal(t, "m", v.Str())
}

func TestProcessTraces_AppliesSourcesInSliceOrder(t *testing.T) {
	// Two sources targeting the same destination; overwrite=true on both.
	// The second source's write wins, confirming iteration order is honored.
	p := &genaiNormalizerProcessor{
		sources: []sourceNormalizer{
			newNormalizer(map[string]string{"src.a": "dst.model"}, true, true),
			newNormalizer(map[string]string{"src.b": "dst.model"}, true, true),
		},
	}

	td, span := newSpan()
	span.Attributes().PutStr("src.a", "from-a")
	span.Attributes().PutStr("src.b", "from-b")

	_, err := p.processTraces(t.Context(), td)
	require.NoError(t, err)

	v, ok := span.Attributes().Get("dst.model")
	require.True(t, ok)
	assert.Equal(t, "from-b", v.Str())
}

func TestProcessTraces_EmptyTraces(t *testing.T) {
	p := &genaiNormalizerProcessor{sources: []sourceNormalizer{
		newNormalizer(map[string]string{"src.a": "dst.a"}, true, false),
	}}
	_, err := p.processTraces(t.Context(), ptrace.NewTraces())
	require.NoError(t, err)
}

func TestCreateTracesProcessor_RejectsInvalidConfig(t *testing.T) {
	_, err := createTracesProcessor(
		t.Context(),
		processortest.NewNopSettings(metadata.Type),
		&Config{Sources: []Source{{Name: "bogus"}}},
		new(consumertest.TracesSink),
	)
	require.Error(t, err)
}

// TestNormalize_OpenInferenceEndToEnd exercises the real OpenInference
// mapping table end-to-end through createTracesProcessor.
func TestNormalize_OpenInferenceEndToEnd(t *testing.T) {
	cfg := &Config{
		Sources: []Source{{Name: SourceOpenInference, RemoveOriginals: true}},
	}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)

	td, span := newSpan()
	span.Attributes().PutInt("llm.token_count.prompt", 100)
	span.Attributes().PutInt("llm.token_count.completion", 200)
	span.Attributes().PutStr("llm.model_name", "claude-sonnet-4")
	span.Attributes().PutStr("llm.provider", "anthropic")
	span.Attributes().PutStr("openinference.span.kind", "LLM")
	span.Attributes().PutStr("agent.name", "research-agent")
	span.Attributes().PutStr("session.id", "sess-123")

	require.NoError(t, p.ConsumeTraces(t.Context(), td))
	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()

	for _, exp := range []struct {
		key  string
		want any
	}{
		{"gen_ai.usage.input_tokens", int64(100)},
		{"gen_ai.usage.output_tokens", int64(200)},
		{"gen_ai.request.model", "claude-sonnet-4"},
		{"gen_ai.provider.name", "anthropic"},
		{"gen_ai.operation.name", "chat"},
		{"gen_ai.agent.name", "research-agent"},
		{"gen_ai.conversation.id", "sess-123"},
	} {
		v, ok := out.Get(exp.key)
		require.True(t, ok, "missing %s", exp.key)
		switch want := exp.want.(type) {
		case int64:
			assert.Equal(t, want, v.Int(), exp.key)
		case string:
			assert.Equal(t, want, v.Str(), exp.key)
		}
	}

	// Originals removed.
	for _, k := range []string{
		"llm.token_count.prompt", "llm.token_count.completion", "llm.model_name",
		"llm.provider", "openinference.span.kind", "agent.name", "session.id",
	} {
		_, ok := out.Get(k)
		assert.False(t, ok, "expected %s to be removed", k)
	}
}
