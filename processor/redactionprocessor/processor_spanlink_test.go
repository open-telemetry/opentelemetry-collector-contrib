// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/metadata"
)

func TestRedactSpanLinkAttributes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		config *Config
		want   map[string]any
	}{
		{
			name:   "default removes all attributes",
			config: &Config{},
			want:   map[string]any{},
		},
		{
			name: "allowlist masking ignored keys and summary",
			config: &Config{
				AllowedKeys:        []string{"email", "api_token", "count"},
				IgnoredKeys:        []string{"trusted"},
				BlockedValues:      []string{`[\w.-]+@[\w.-]+`},
				BlockedKeyPatterns: []string{"token"},
				Summary:            debug,
			},
			want: map[string]any{
				"email": "****", "api_token": "****", "count": int64(7),
				"trusted":             "trusted@example.test",
				redactionRedactedKeys: "nested,remove", redactionRedactedCount: int64(2),
				redactionMaskedKeys: "api_token,email", redactionMaskedCount: int64(2),
				redactionIgnoredCount: int64(1),
			},
		},
		{
			name: "allow all preserves non-string values by default",
			config: &Config{
				AllowAllKeys:       true,
				BlockedValues:      []string{`[\w.-]+@[\w.-]+`},
				AllowedValues:      []string{`^trusted@`},
				BlockedKeyPatterns: []string{"token"},
			},
			want: map[string]any{
				"email": "****", "api_token": "****", "count": int64(7),
				"trusted": "trusted@example.test", "remove": "not allowed",
				"nested": map[string]any{"email": "nested@example.test"},
			},
		},
		{
			name: "redact all types uses string representation",
			config: &Config{
				AllowAllKeys:       true,
				RedactAllTypes:     true,
				BlockedValues:      []string{`[\w.-]+@[\w.-]+`},
				AllowedValues:      []string{`^trusted@`},
				BlockedKeyPatterns: []string{"token"},
			},
			want: map[string]any{
				"email": "****", "api_token": "****", "count": int64(7),
				"trusted": "trusted@example.test", "remove": "not allowed",
				"nested": `{"email":"****"}`,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := ptrace.NewTraces()
			for range 2 {
				spans := input.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
				spans.AppendEmpty().SetName("no links")
				span := spans.AppendEmpty()
				span.SetName("linked operation")
				for range 2 {
					link := span.Links().AppendEmpty()
					link.SetTraceID(pcommon.TraceID{1, 2, 3})
					link.SetSpanID(pcommon.SpanID{4, 5, 6})
					link.TraceState().FromRaw("vendor=value")
					link.SetFlags(1)
					link.SetDroppedAttributesCount(3)
					attrs := link.Attributes()
					attrs.PutStr("email", "person@example.test")
					attrs.PutStr("api_token", "synthetic-token\nsecond-line")
					attrs.PutStr("trusted", "trusted@example.test")
					attrs.PutStr("remove", "not allowed")
					attrs.PutInt("count", 7)
					attrs.PutEmptyMap("nested").PutStr("email", "nested@example.test")
				}
			}

			sink := new(consumertest.TracesSink)
			processor, err := NewFactory().CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), tt.config, sink)
			require.NoError(t, err)
			require.NoError(t, processor.Start(t.Context(), componenttest.NewNopHost()))
			t.Cleanup(func() { require.NoError(t, processor.Shutdown(t.Context())) })
			require.NoError(t, processor.ConsumeTraces(t.Context(), input))
			require.Len(t, sink.AllTraces(), 1)
			output := sink.AllTraces()[0]
			require.Equal(t, 2, output.ResourceSpans().Len())
			for i := 0; i < output.ResourceSpans().Len(); i++ {
				spans := output.ResourceSpans().At(i).ScopeSpans().At(0).Spans()
				require.Equal(t, 2, spans.Len())
				assert.Equal(t, 0, spans.At(0).Links().Len())
				span := spans.At(1)
				assert.Equal(t, "linked operation", span.Name())
				require.Equal(t, 2, span.Links().Len())
				for j := 0; j < span.Links().Len(); j++ {
					link := span.Links().At(j)
					assert.Equal(t, tt.want, link.Attributes().AsRaw())
					assert.Equal(t, pcommon.TraceID{1, 2, 3}, link.TraceID())
					assert.Equal(t, pcommon.SpanID{4, 5, 6}, link.SpanID())
					assert.Equal(t, "vendor=value", link.TraceState().AsRaw())
					assert.Equal(t, uint32(1), link.Flags())
					assert.Equal(t, uint32(3), link.DroppedAttributesCount())
				}
			}
		})
	}
}

func TestRedactSpanLinksPreservesTrace(t *testing.T) {
	t.Parallel()
	// A fixed HMAC-SHA256 test vector for key strings.Repeat("k", 32) and
	// message "redact-me", computed independently of the processor.
	const digest = "338b64379055fffcb89b4dba9ce08d473a281794f950e3d6b12d9ebf8d6aff9d"
	input := ptrace.NewTraces()
	input.ResourceSpans().AppendEmpty() // Empty resource and scope are valid.
	for r := range 2 {
		rs := input.ResourceSpans().AppendEmpty()
		rs.SetSchemaUrl("https://example.test/resource")
		rs.Resource().SetDroppedAttributesCount(5)
		rs.Resource().Attributes().PutStr("safe", "resource")
		rs.ScopeSpans().AppendEmpty()
		for s := range 2 {
			ss := rs.ScopeSpans().AppendEmpty()
			ss.SetSchemaUrl("https://example.test/scope")
			ss.Scope().SetName("instrumentation")
			ss.Scope().SetVersion("1.0")
			ss.Scope().Attributes().PutBool("safe", true)
			for n := range 3 {
				span := ss.Spans().AppendEmpty()
				span.SetName(fmt.Sprintf("operation-%d-%d-%d", r, s, n))
				span.SetTraceID(pcommon.TraceID{1, byte(r + 1), byte(s + 1)})
				span.SetSpanID(pcommon.SpanID{2, byte(n + 1)})
				span.SetParentSpanID(pcommon.SpanID{3})
				span.TraceState().FromRaw("vendor=parent")
				span.SetKind(ptrace.SpanKindClient)
				span.SetFlags(257)
				span.SetStartTimestamp(1)
				span.SetEndTimestamp(2)
				span.SetDroppedAttributesCount(3)
				span.SetDroppedEventsCount(4)
				span.SetDroppedLinksCount(5)
				span.Status().SetCode(ptrace.StatusCodeError)
				span.Status().SetMessage("status redact-me") // Not an attribute.
				span.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{0, 255})
				span.Attributes().PutDouble("double", 1.25)
				event := span.Events().AppendEmpty()
				event.SetName("event redact-me") // Not an attribute.
				event.SetTimestamp(1)
				event.SetDroppedAttributesCount(6)
				event.Attributes().PutInt("safe", 42)
				for l := range n {
					link := span.Links().AppendEmpty()
					link.SetTraceID(pcommon.TraceID{byte(r + 1), byte(s + 1), byte(n + 1), byte(l + 1)})
					link.SetSpanID(pcommon.SpanID{byte(n + 1), byte(l + 1)})
					link.TraceState().FromRaw(fmt.Sprintf("vendor=link%d", l))
					link.SetFlags(uint32(l + 256))
					link.SetDroppedAttributesCount(uint32(l + 1))
					link.Attributes().PutStr("secret", "prefix redact-me suffix")
					// String-only masking must not rewrite these container values.
					link.Attributes().PutEmptySlice("slice").AppendEmpty().SetStr("redact-me")
					link.Attributes().PutEmptyMap("map").PutStr("nested", "redact-me")
				}
			}
			ss.Spans().AppendEmpty().Links().AppendEmpty() // Empty link attributes.
		}
	}

	expected := ptrace.NewTraces()
	input.CopyTo(expected)
	for _, rs := range expected.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				for _, link := range span.Links().All() {
					if _, ok := link.Attributes().Get("secret"); ok {
						link.Attributes().PutStr("secret", "prefix "+digest+" suffix")
					}
				}
			}
		}
	}
	processor, err := newRedaction(t.Context(), &Config{
		AllowAllKeys:  true,
		BlockedValues: []string{"redact-me"},
		HashFunction:  HMACSHA256,
		HMACKey:       configopaque.String(strings.Repeat("k", 32)),
	}, zaptest.NewLogger(t))
	require.NoError(t, err)
	output, err := processor.processTraces(t.Context(), input)
	require.NoError(t, err)
	marshaler := ptrace.ProtoMarshaler{}
	want, err := marshaler.MarshalTraces(expected)
	require.NoError(t, err)
	got, err := marshaler.MarshalTraces(output)
	require.NoError(t, err)
	assert.Equal(t, want, got, "only the matching link values should change")
}
