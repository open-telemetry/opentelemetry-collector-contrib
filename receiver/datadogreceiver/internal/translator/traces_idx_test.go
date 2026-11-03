// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace/idx"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

// legacyTracerPayload returns a payload in the legacy format exercising the fields that the Datadog
// Agent moves around when converting to the string-table ("idx") format.
func legacyTracerPayload() *pb.TracerPayload {
	return &pb.TracerPayload{
		ContainerID:     "container-id",
		LanguageName:    "go",
		LanguageVersion: "1.25",
		TracerVersion:   "2.3.0",
		Env:             "prod",
		Hostname:        "host",
		AppVersion:      "1.0.1",
		Chunks: []*pb.TraceChunk{{
			Priority: 2,
			Origin:   "lambda",
			Spans: []*pb.Span{
				{
					// Child span first: Datadog does not order spans root-first.
					Service:  "my-service",
					Name:     "postgresql.query",
					Resource: "SELECT 1",
					Type:     "sql",
					TraceID:  0x1d99f09757cf199d,
					SpanID:   2,
					ParentID: 1,
					Start:    1000,
					Duration: 100,
					Error:    1,
					Meta: map[string]string{
						"db.operation": "SELECT",
						"span.kind":    "client",
						"env":          "prod",
						"version":      "1.0.1",
					},
					Metrics: map[string]float64{"rows": 3},
				},
				{
					Service:  "my-service",
					Name:     "http.request",
					Resource: "GET /users",
					Type:     "web",
					TraceID:  0x1d99f09757cf199d,
					SpanID:   1,
					ParentID: 0x6b953724b399048a,
					Start:    900,
					Duration: 300,
					Meta: map[string]string{
						"_dd.p.tid":   "f233b7e1421e8bde",
						"http.method": "GET",
						"http.route":  "/users",
						"span.kind":   "server",
						"component":   "net/http",
						"env":         "prod",
					},
					Metrics: map[string]float64{"_sampling_priority_v1": 2},
					SpanLinks: []*pb.SpanLink{{
						TraceID:     0x1d99f09757cf199d,
						TraceIDHigh: 0xf233b7e1421e8bde,
						SpanID:      0x5ab19b8ebe922796,
						Tracestate:  "dd=s:1",
						Flags:       1,
						Attributes:  map[string]string{"link.reason": "batch"},
					}},
					SpanEvents: []*pb.SpanEvent{{
						TimeUnixNano: 950,
						Name:         "exception",
						Attributes: map[string]*pb.AttributeAnyValue{
							"exception.message": {Type: pb.AttributeAnyValue_STRING_VALUE, StringValue: "boom"},
							"exception.count":   {Type: pb.AttributeAnyValue_INT_VALUE, IntValue: 2},
						},
					}},
				},
			},
		}},
	}
}

// stringTable builds the string table of an idx.TracerPayload, index 0 being the empty string.
type stringTable struct {
	strings []string
	refs    map[string]uint32
}

func (st *stringTable) ref(s string) uint32 {
	if st.refs == nil {
		st.strings = []string{""}
		st.refs = map[string]uint32{"": 0}
	}
	if ref, ok := st.refs[s]; ok {
		return ref
	}
	ref := uint32(len(st.strings))
	st.strings = append(st.strings, s)
	st.refs[s] = ref
	return ref
}

func (st *stringTable) str(s string) *idx.AnyValue {
	return &idx.AnyValue{Value: &idx.AnyValue_StringValueRef{StringValueRef: st.ref(s)}}
}

func double(f float64) *idx.AnyValue {
	return &idx.AnyValue{Value: &idx.AnyValue_DoubleValue{DoubleValue: f}}
}

// idxTracerPayload returns legacyTracerPayload in the string-table ("idx") format, the way the Datadog
// Agent converts it: env, version, component and span.kind are promoted to span fields and removed
// from the attributes, and the 128-bit trace ID is set on the chunk. _dd.p.tid is left out of the
// attributes to check it is restored from the chunk trace ID.
func idxTracerPayload() *idx.TracerPayload {
	st := &stringTable{}
	traceID := []byte{0xf2, 0x33, 0xb7, 0xe1, 0x42, 0x1e, 0x8b, 0xde, 0x1d, 0x99, 0xf0, 0x97, 0x57, 0xcf, 0x19, 0x9d}
	return &idx.TracerPayload{
		ContainerIDRef:     st.ref("container-id"),
		LanguageNameRef:    st.ref("go"),
		LanguageVersionRef: st.ref("1.25"),
		TracerVersionRef:   st.ref("2.3.0"),
		EnvRef:             st.ref("prod"),
		HostnameRef:        st.ref("host"),
		AppVersionRef:      st.ref("1.0.1"),
		Chunks: []*idx.TraceChunk{{
			Priority:  2,
			OriginRef: st.ref("lambda"),
			TraceID:   traceID,
			Spans: []*idx.Span{
				{
					ServiceRef:  st.ref("my-service"),
					NameRef:     st.ref("postgresql.query"),
					ResourceRef: st.ref("SELECT 1"),
					TypeRef:     st.ref("sql"),
					SpanID:      2,
					ParentID:    1,
					Start:       1000,
					Duration:    100,
					Error:       true,
					EnvRef:      st.ref("prod"),
					VersionRef:  st.ref("1.0.1"),
					Kind:        idx.SpanKind_SPAN_KIND_CLIENT,
					Attributes: map[uint32]*idx.AnyValue{
						st.ref("db.operation"): st.str("SELECT"),
						st.ref("rows"):         double(3),
					},
				},
				{
					ServiceRef:   st.ref("my-service"),
					NameRef:      st.ref("http.request"),
					ResourceRef:  st.ref("GET /users"),
					TypeRef:      st.ref("web"),
					SpanID:       1,
					ParentID:     0x6b953724b399048a,
					Start:        900,
					Duration:     300,
					EnvRef:       st.ref("prod"),
					ComponentRef: st.ref("net/http"),
					Kind:         idx.SpanKind_SPAN_KIND_SERVER,
					Attributes: map[uint32]*idx.AnyValue{
						st.ref("http.method"):           st.str("GET"),
						st.ref("http.route"):            st.str("/users"),
						st.ref("_sampling_priority_v1"): double(2),
					},
					Links: []*idx.SpanLink{{
						TraceID:       traceID,
						SpanID:        0x5ab19b8ebe922796,
						TracestateRef: st.ref("dd=s:1"),
						Flags:         1,
						Attributes:    map[uint32]*idx.AnyValue{st.ref("link.reason"): st.str("batch")},
					}},
					Events: []*idx.SpanEvent{{
						Time:    950,
						NameRef: st.ref("exception"),
						Attributes: map[uint32]*idx.AnyValue{
							st.ref("exception.message"): st.str("boom"),
							st.ref("exception.count"):   {Value: &idx.AnyValue_IntValue{IntValue: 2}},
						},
					}},
				},
			},
		}},
		Strings: st.strings,
	}
}

func newTracesRequest(t *testing.T, body []byte) *http.Request {
	req, err := http.NewRequest(http.MethodPost, "/api/v0.2/traces", io.NopCloser(bytes.NewReader(body)))
	require.NoError(t, err)
	return req
}

func toTraces(t *testing.T, payloads []*pb.TracerPayload, req *http.Request) ptrace.Traces {
	cache, err := lru.New[uint64, pcommon.TraceID](10)
	require.NoError(t, err)
	traces := ptrace.NewTraces()
	for _, payload := range payloads {
		translated, err := ToTraces(zap.NewNop(), payload, req, cache)
		require.NoError(t, err)
		translated.ResourceSpans().MoveAndAppendTo(traces.ResourceSpans())
	}
	return traces
}

// TestTracePayloadApiV02IdxUnmarshalling verifies that traces sent by the Datadog Agent 7.73+ in
// AgentPayload.IdxTracerPayloads translate to the same OTLP traces as the legacy TracerPayloads.
func TestTracePayloadApiV02IdxUnmarshalling(t *testing.T) {
	legacyBody, err := proto.Marshal(&pb.AgentPayload{TracerPayloads: []*pb.TracerPayload{legacyTracerPayload()}})
	require.NoError(t, err)
	legacyReq := newTracesRequest(t, legacyBody)
	legacyPayloads, err := HandleTracesPayload(legacyReq)
	require.NoError(t, err)
	expected := toTraces(t, legacyPayloads, legacyReq)
	require.Equal(t, 2, expected.SpanCount())

	// Serialize the way the Agent's v1 trace writer does.
	body, err := pb.MarshalAgentPayload(&pb.AgentPayload{IdxTracerPayloads: []*idx.TracerPayload{idxTracerPayload()}})
	require.NoError(t, err)

	req := newTracesRequest(t, body)
	payloads, err := HandleTracesPayload(req)
	require.NoError(t, err)
	require.Len(t, payloads, 1)
	actual := toTraces(t, payloads, req)

	require.NoError(t, ptracetest.CompareTraces(expected, actual,
		ptracetest.IgnoreResourceSpansOrder(), ptracetest.IgnoreSpansOrder()))

	span := actual.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, "f233b7e1421e8bde1d99f09757cf199d", span.TraceID().String())
}

// TestTracePayloadApiV02BothPayloadFormats verifies that an AgentPayload carrying both the legacy and
// the idx tracer payloads yields both.
func TestTracePayloadApiV02BothPayloadFormats(t *testing.T) {
	legacy := legacyTracerPayload()
	legacy.LanguageName = "python"
	agentPayload := &pb.AgentPayload{
		TracerPayloads:    []*pb.TracerPayload{legacy},
		IdxTracerPayloads: []*idx.TracerPayload{idxTracerPayload()},
	}
	body, err := proto.Marshal(agentPayload)
	require.NoError(t, err)

	payloads, err := HandleTracesPayload(newTracesRequest(t, body))
	require.NoError(t, err)
	require.Len(t, payloads, 2)
	assert.Equal(t, "python", payloads[0].LanguageName)
	assert.Equal(t, "go", payloads[1].LanguageName)
	for _, payload := range payloads {
		require.Len(t, payload.Chunks, 1)
		assert.Len(t, payload.Chunks[0].Spans, 2)
	}
}

func TestTracerPayloadFromIdx(t *testing.T) {
	traceID := []byte{0xf2, 0x33, 0xb7, 0xe1, 0x42, 0x1e, 0x8b, 0xde, 0x1d, 0x99, 0xf0, 0x97, 0x57, 0xcf, 0x19, 0x9d}
	strs := []string{"", "svc", "op", "res", "web", "attr", "value", "prod", "1.2.3", "grpc", "evt", "list", "dd=s:1"}
	payload := &idx.TracerPayload{
		Strings:         strs,
		LanguageNameRef: 9,
		Attributes: map[uint32]*idx.AnyValue{
			5: {Value: &idx.AnyValue_IntValue{IntValue: 42}},
		},
		Chunks: []*idx.TraceChunk{{
			Priority: 1,
			TraceID:  traceID,
			Spans: []*idx.Span{{
				ServiceRef:   1,
				NameRef:      2,
				ResourceRef:  3,
				TypeRef:      4,
				SpanID:       7,
				ParentID:     0,
				Start:        10,
				Duration:     5,
				Error:        true,
				EnvRef:       7,
				VersionRef:   8,
				ComponentRef: 9,
				Kind:         idx.SpanKind_SPAN_KIND_INTERNAL,
				Attributes: map[uint32]*idx.AnyValue{
					5:  {Value: &idx.AnyValue_StringValueRef{StringValueRef: 6}},
					2:  {Value: &idx.AnyValue_BoolValue{BoolValue: true}},
					3:  {Value: &idx.AnyValue_DoubleValue{DoubleValue: 1.5}},
					4:  {Value: &idx.AnyValue_BytesValue{BytesValue: []byte("raw")}},
					11: {Value: &idx.AnyValue_ArrayValue{ArrayValue: &idx.ArrayValue{Values: []*idx.AnyValue{{Value: &idx.AnyValue_StringValueRef{StringValueRef: 6}}, {Value: &idx.AnyValue_IntValue{IntValue: 1}}}}}},
				},
				Links: []*idx.SpanLink{{TraceID: traceID, SpanID: 3, TracestateRef: 12, Flags: 1}},
				Events: []*idx.SpanEvent{{
					Time:    11,
					NameRef: 10,
					Attributes: map[uint32]*idx.AnyValue{
						5: {Value: &idx.AnyValue_KeyValueList{KeyValueList: &idx.KeyValueList{KeyValues: []*idx.KeyValue{{Key: 5, Value: &idx.AnyValue{Value: &idx.AnyValue_StringValueRef{StringValueRef: 6}}}}}}},
					},
				}},
			}},
		}},
	}

	tp, err := tracerPayloadFromIdx(payload)
	require.NoError(t, err)
	assert.Equal(t, "grpc", tp.LanguageName)
	assert.Equal(t, map[string]string{"attr": "42"}, tp.Tags)
	require.Len(t, tp.Chunks, 1)
	assert.Equal(t, int32(1), tp.Chunks[0].Priority)
	require.Len(t, tp.Chunks[0].Spans, 1)

	span := tp.Chunks[0].Spans[0]
	assert.Equal(t, "svc", span.Service)
	assert.Equal(t, "op", span.Name)
	assert.Equal(t, "res", span.Resource)
	assert.Equal(t, "web", span.Type)
	assert.Equal(t, uint64(0x1d99f09757cf199d), span.TraceID)
	assert.Equal(t, int32(1), span.Error)
	assert.Equal(t, map[string]string{
		"attr":      "value",
		"op":        "true",
		"list":      `["value",1]`,
		"env":       "prod",
		"version":   "1.2.3",
		"component": "grpc",
		"_dd.p.tid": "f233b7e1421e8bde",
	}, span.Meta)
	assert.Equal(t, map[string]float64{"res": 1.5}, span.Metrics)
	assert.Equal(t, map[string][]byte{"web": []byte("raw")}, span.MetaStruct)

	require.Len(t, span.SpanLinks, 1)
	assert.Equal(t, uint64(0xf233b7e1421e8bde), span.SpanLinks[0].TraceIDHigh)
	assert.Equal(t, uint64(0x1d99f09757cf199d), span.SpanLinks[0].TraceID)
	assert.Equal(t, "dd=s:1", span.SpanLinks[0].Tracestate)

	require.Len(t, span.SpanEvents, 1)
	assert.Equal(t, "evt", span.SpanEvents[0].Name)
	assert.Equal(t, `{"attr":"value"}`, span.SpanEvents[0].Attributes["attr"].StringValue)
}

func TestTracerPayloadFromIdxInvalidStringRef(t *testing.T) {
	payload := &idx.TracerPayload{
		Strings: []string{"", "svc"},
		Chunks:  []*idx.TraceChunk{{Spans: []*idx.Span{{ServiceRef: 1, NameRef: 5}}}},
	}
	_, err := tracerPayloadFromIdx(payload)
	require.ErrorContains(t, err, "string table reference 5 out of range")

	body, err := proto.Marshal(&pb.AgentPayload{IdxTracerPayloads: []*idx.TracerPayload{payload}})
	require.NoError(t, err)
	_, err = HandleTracesPayload(newTracesRequest(t, body))
	require.Error(t, err)
}

// TestTracePayloadApiV02IdxRecorded decodes a payload recorded from Datadog Agent 7.85.0 forwarding a
// v0.4 trace (two spans, 128-bit trace ID) to an additional APM endpoint.
func TestTracePayloadApiV02IdxRecorded(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "agent-7.85.0-idx-traces.pb"))
	require.NoError(t, err)

	req := newTracesRequest(t, body)
	payloads, err := HandleTracesPayload(req)
	require.NoError(t, err)
	require.Len(t, payloads, 1)

	traces := toTraces(t, payloads, req)
	require.Equal(t, 2, traces.SpanCount())
	kinds := map[string]ptrace.SpanKind{}
	for _, rs := range traces.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				assert.Equal(t, "6aea000000000000000000000000007b", span.TraceID().String())
				kinds[span.Name()] = span.Kind()
			}
		}
	}
	assert.Equal(t, map[string]ptrace.SpanKind{
		"servlet.request": ptrace.SpanKindServer,
		"postgresql":      ptrace.SpanKindClient,
	}, kinds)
}
