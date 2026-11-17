// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"net/http"
	"testing"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace/idx"
	ddsampler "github.com/DataDog/datadog-agent/pkg/trace/sampler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestToTracesV1SamplingPriority(t *testing.T) {
	for name, tc := range map[string]struct {
		priority int32
		want     any
	}{
		"user keep":   {priority: int32(ddsampler.PriorityUserKeep), want: "2.000000"},
		"auto reject": {priority: int32(ddsampler.PriorityAutoDrop), want: "0.000000"},
		"user reject": {priority: int32(ddsampler.PriorityUserDrop), want: "-1.000000"},
		"unset":       {priority: int32(ddsampler.PriorityNone), want: nil},
	} {
		t.Run(name, func(t *testing.T) {
			strs := idx.NewStringTable()
			span := idx.NewInternalSpan(strs, &idx.Span{ServiceRef: strs.Add("svc"), SpanID: 1})
			payload := &idx.InternalTracerPayload{
				Strings: strs,
				Chunks:  []*idx.InternalTraceChunk{idx.NewInternalTraceChunk(strs, tc.priority, "", nil, []*idx.InternalSpan{span}, false, make([]byte, 16), 0)},
			}

			traces, err := ToTracesV1(zap.NewNop(), payload, &http.Request{Header: http.Header{}})
			require.NoError(t, err)

			attrs := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().AsRaw()
			assert.Equal(t, tc.want, attrs["sampling.priority"])
		})
	}
}

// Span links and events can be JSON tags, for spans from tracers and for spans that the agent converted from a v0.x
// format. A converted span has the converter's marker and the internal kind when the tracer did not set one.
func TestToTracesV1TagsAndConvertedSpans(t *testing.T) {
	links := `[{"trace_id":"00000000000000000000000000000abc","span_id":"0000000000000def","flags":1}]`
	events := `[{"time_unix_nano":5,"name":"boom","attributes":{"k":"v"}}]`

	for name, tc := range map[string]struct {
		converted bool
		kind      idx.SpanKind
		spanType  string
		wantKind  ptrace.SpanKind
	}{
		"converted web span without span.kind":  {converted: true, kind: idx.SpanKind_SPAN_KIND_INTERNAL, spanType: "web", wantKind: ptrace.SpanKindServer},
		"converted http span without span.kind": {converted: true, kind: idx.SpanKind_SPAN_KIND_INTERNAL, spanType: "http", wantKind: ptrace.SpanKindClient},
		"converted internal span":               {converted: true, kind: idx.SpanKind_SPAN_KIND_INTERNAL, spanType: "custom", wantKind: ptrace.SpanKindInternal},
		"converted span with span.kind":         {converted: true, kind: idx.SpanKind_SPAN_KIND_CLIENT, spanType: "web", wantKind: ptrace.SpanKindClient},
		"native internal web span":              {converted: false, kind: idx.SpanKind_SPAN_KIND_INTERNAL, spanType: "web", wantKind: ptrace.SpanKindInternal},
	} {
		t.Run(name, func(t *testing.T) {
			strs := idx.NewStringTable()
			str := func(s string) *idx.AnyValue {
				return &idx.AnyValue{Value: &idx.AnyValue_StringValueRef{StringValueRef: strs.Add(s)}}
			}
			attrs := map[uint32]*idx.AnyValue{
				strs.Add("_dd.span_links"): str(links),
				strs.Add("events"):         str(events),
			}
			if tc.converted {
				attrs[strs.Add(attributeConvertedV1)] = str("v04")
			}
			span := idx.NewInternalSpan(strs, &idx.Span{ServiceRef: strs.Add("svc"), TypeRef: strs.Add(tc.spanType), Kind: tc.kind, SpanID: 2, Attributes: attrs})
			payload := &idx.InternalTracerPayload{
				Strings: strs,
				Chunks:  []*idx.InternalTraceChunk{idx.NewInternalTraceChunk(strs, 1, "", nil, []*idx.InternalSpan{span}, false, make([]byte, 16), 0)},
			}

			traces, err := ToTracesV1(zap.NewNop(), payload, &http.Request{Header: http.Header{}})
			require.NoError(t, err)

			got := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, tc.wantKind, got.Kind())
			require.Equal(t, 1, got.Links().Len())
			link := got.Links().At(0)
			assert.Equal(t, pcommon.TraceID{15: 0xbc, 14: 0x0a}, link.TraceID())
			assert.Equal(t, pcommon.SpanID{7: 0xef, 6: 0x0d}, link.SpanID())
			assert.Equal(t, uint32(1), link.Flags())
			require.Equal(t, 1, got.Events().Len())
			assert.Equal(t, "boom", got.Events().At(0).Name())
			assert.Equal(t, pcommon.Timestamp(5), got.Events().At(0).Timestamp())
			assert.Equal(t, map[string]any{"k": "v"}, got.Events().At(0).Attributes().AsRaw())
			assert.NotContains(t, got.Attributes().AsRaw(), "_dd.span_links")
			assert.NotContains(t, got.Attributes().AsRaw(), "events")
			if tc.converted {
				assert.Equal(t, "v04", got.Attributes().AsRaw()[attributeConvertedV1], "the marker stays")
			}
		})
	}
}

func v1PayloadWithSpan(span *idx.Span, strs *idx.StringTable) *idx.InternalTracerPayload {
	return &idx.InternalTracerPayload{
		Strings: strs,
		Chunks: []*idx.InternalTraceChunk{
			idx.NewInternalTraceChunk(strs, 1, "", nil, []*idx.InternalSpan{idx.NewInternalSpan(strs, span)}, false, make([]byte, 16), 0),
		},
	}
}

// The v1 format only says whether a span failed, not with which status code, so a failed gRPC span has no status code.
func TestToTracesV1GRPCStatusCode(t *testing.T) {
	for name, tc := range map[string]struct {
		failed bool
		want   any
	}{
		"ok span":     {failed: false, want: "0"},
		"failed span": {failed: true, want: nil},
	} {
		t.Run(name, func(t *testing.T) {
			strs := idx.NewStringTable()
			span := &idx.Span{
				ServiceRef: strs.Add("svc"), NameRef: strs.Add("grpc.server"), ResourceRef: strs.Add("/pkg.Service/Method"),
				Kind: idx.SpanKind_SPAN_KIND_SERVER, SpanID: 1, Error: tc.failed,
			}

			traces, err := ToTracesV1(zap.NewNop(), v1PayloadWithSpan(span, strs), &http.Request{Header: http.Header{}})
			require.NoError(t, err)

			attrs := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().AsRaw()
			assert.Equal(t, "grpc", attrs["rpc.system.name"], "the span is still processed as a gRPC span")
			assert.Equal(t, tc.want, attrs["rpc.response.status_code"])
		})
	}
}

// Payloads that an agent forwards are not checked when they are decoded, so a reference to a string that does not exist
// is possible.
func TestToTracesV1StringReferenceOutOfRange(t *testing.T) {
	strs := idx.NewStringTable()
	span := &idx.Span{ServiceRef: 99, SpanID: 1}

	_, err := ToTracesV1(zap.NewNop(), v1PayloadWithSpan(span, strs), &http.Request{Header: http.Header{}})

	require.Error(t, err)
}

// convertedByAgent decodes a v0.4 trace the way the Datadog Agent does when it converts traces to the v1 format.
func convertedByAgent(t *testing.T, span *pb.Span) *idx.InternalTracerPayload {
	t.Helper()
	body, err := pb.Traces{pb.Trace{span}}.MarshalMsg(nil)
	require.NoError(t, err)
	var payload idx.InternalTracerPayload
	_, err = payload.UnmarshalMsgConverted(body)
	require.NoError(t, err)
	return &payload
}

// These are the spans as the agent's converter produces them, not as we expect it to. It sets the internal kind on
// spans without a span.kind and keeps the tags that it also promotes to fields.
func TestToTracesV1ConvertedByAgent(t *testing.T) {
	links := `[{"trace_id":"00000000000000000000000000000abc","span_id":"0000000000000def"}]`
	events := `[{"time_unix_nano":5,"name":"boom","attributes":{"k":"v"}}]`

	for name, tc := range map[string]struct {
		spanType string
		kindTag  string
		wantKind ptrace.SpanKind
	}{
		"web span without span.kind":       {spanType: "web", wantKind: ptrace.SpanKindServer},
		"http span without span.kind":      {spanType: "http", wantKind: ptrace.SpanKindClient},
		"other span without span.kind":     {spanType: "custom", wantKind: ptrace.SpanKindInternal},
		"web span with span.kind internal": {spanType: "web", kindTag: "internal", wantKind: ptrace.SpanKindInternal},
		"web span with span.kind client":   {spanType: "web", kindTag: "client", wantKind: ptrace.SpanKindClient},
	} {
		t.Run(name, func(t *testing.T) {
			meta := map[string]string{
				"_dd.span_links": links,
				"events":         events,
				"_dd.origin":     "synthetics",
				"_dd.p.dm":       "-4",
			}
			if tc.kindTag != "" {
				meta["span.kind"] = tc.kindTag
			}
			payload := convertedByAgent(t, &pb.Span{Service: "svc", Name: "n", TraceID: 1, SpanID: 2, Type: tc.spanType, Meta: meta})

			traces, err := ToTracesV1(zap.NewNop(), payload, &http.Request{Header: http.Header{}})
			require.NoError(t, err)

			got := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, tc.wantKind, got.Kind())
			assert.Equal(t, 1, got.Links().Len())
			assert.Equal(t, 1, got.Events().Len())
			attrs := got.Attributes().AsRaw()
			assert.NotContains(t, attrs, "_dd.span_links")
			assert.NotContains(t, attrs, "events")
			assert.Equal(t, "synthetics", attrs["_dd.origin"])
			assert.Equal(t, "-4", attrs["_dd.p.dm"])
			assert.Equal(t, "v04", attrs[attributeConvertedV1])
			if tc.kindTag != "" {
				assert.Equal(t, tc.kindTag, attrs["span.kind"], "the agent keeps an explicit span.kind as an attribute")
			}
		})
	}
}

// In the v1 format the origin, the sampling decision maker and the tags are on the chunk. The older formats have them as
// span attributes.
func TestToTracesV1ChunkFields(t *testing.T) {
	for name, tc := range map[string]struct {
		origin    string
		mechanism uint32
		want      map[string]any
		wantNone  []string
	}{
		"all set": {
			origin: "synthetics", mechanism: 4,
			want: map[string]any{"_dd.origin": "synthetics", "_dd.p.dm": "-4", "_dd.p.team": "apm", "shared": "span"},
		},
		"none set": {
			want:     map[string]any{"_dd.p.team": "apm", "shared": "span"},
			wantNone: []string{"_dd.origin", "_dd.p.dm"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			strs := idx.NewStringTable()
			str := func(s string) *idx.AnyValue {
				return &idx.AnyValue{Value: &idx.AnyValue_StringValueRef{StringValueRef: strs.Add(s)}}
			}
			chunkAttrs := map[uint32]*idx.AnyValue{strs.Add("_dd.p.team"): str("apm"), strs.Add("shared"): str("chunk")}
			span := idx.NewInternalSpan(strs, &idx.Span{
				ServiceRef: strs.Add("svc"), SpanID: 1,
				Attributes: map[uint32]*idx.AnyValue{strs.Add("shared"): str("span")},
			})
			payload := &idx.InternalTracerPayload{
				Strings: strs,
				Chunks: []*idx.InternalTraceChunk{
					idx.NewInternalTraceChunk(strs, 1, tc.origin, chunkAttrs, []*idx.InternalSpan{span}, false, make([]byte, 16), tc.mechanism),
				},
			}

			traces, err := ToTracesV1(zap.NewNop(), payload, &http.Request{Header: http.Header{}})
			require.NoError(t, err)

			attrs := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().AsRaw()
			for k, v := range tc.want {
				assert.Equal(t, v, attrs[k], k)
			}
			for _, k := range tc.wantNone {
				assert.NotContains(t, attrs, k)
			}
		})
	}
}
