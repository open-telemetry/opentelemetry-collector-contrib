// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinv1

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	zipkinmodel "github.com/openzipkin/zipkin-go/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/internal/zipkin"
)

func TestErrorSpanToTraces(t *testing.T) {
	blob, err := os.ReadFile("./testdata/zipkin_v1_error_batch.json")
	require.NoError(t, err, "Failed to load test data")

	_, err = NewJSONTracesUnmarshaler(false).UnmarshalTraces(blob)
	assert.Error(t, err, "Should have generated error")
}

func TestHexIDToSpanID(t *testing.T) {
	tests := []struct {
		name    string
		hexStr  string
		want    pcommon.SpanID
		wantErr error
	}{
		{
			name:    "empty hex string",
			hexStr:  "",
			wantErr: errHexIDWrongLen,
		},
		{
			name:    "wrong length",
			hexStr:  "0000",
			wantErr: errHexIDWrongLen,
		},
		{
			name:    "parse error",
			hexStr:  "000000000000000-",
			wantErr: hex.InvalidByteError(0x2d),
		},
		{
			name:    "all zero",
			hexStr:  "0000000000000000",
			wantErr: errHexIDZero,
		},
		{
			name:    "happy path",
			hexStr:  "0706050400010203",
			want:    [8]byte{7, 6, 5, 4, 0, 1, 2, 3},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hexToSpanID(tt.hexStr)
			require.Equal(t, tt.wantErr, err)
			if tt.wantErr != nil {
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHexToTraceID(t *testing.T) {
	tests := []struct {
		name    string
		hexStr  string
		want    [16]byte
		wantErr error
	}{
		{
			name:    "empty hex string",
			hexStr:  "",
			wantErr: errHexTraceIDWrongLen,
		},
		{
			name:    "wrong length",
			hexStr:  "000000000000000010",
			wantErr: errHexTraceIDWrongLen,
		},
		{
			name:    "parse error",
			hexStr:  "000000000000000X0000000000000000",
			wantErr: hex.InvalidByteError(0x58),
		},
		{
			name:    "all zero",
			hexStr:  "00000000000000000000000000000000",
			wantErr: errHexTraceIDZero,
		},
		{
			name:    "happy path",
			hexStr:  "00000000000000010000000000000002",
			want:    [16]byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 2},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hexToTraceID(tt.hexStr)
			require.Equal(t, tt.wantErr, err)
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestZipkinJSONFallbackToLocalComponent(t *testing.T) {
	blob, err := os.ReadFile("./testdata/zipkin_v1_local_component.json")
	require.NoError(t, err, "Failed to load test data")

	reqs, err := jsonBatchToTraces(blob, false)
	require.NoError(t, err)
	require.Equal(t, 2, reqs.ResourceSpans().Len(), "Invalid trace service requests count")

	// First span didn't have a host/endpoint to give service name, use the local component.
	gotFirst, found := reqs.ResourceSpans().At(0).Resource().Attributes().Get(conventions.AttributeServiceName)
	require.True(t, found)

	gotSecond, found := reqs.ResourceSpans().At(1).Resource().Attributes().Get(conventions.AttributeServiceName)
	require.True(t, found)

	if gotFirst.AsString() == "myLocalComponent" {
		require.Equal(t, "myLocalComponent", gotFirst.Str())
		require.Equal(t, "myServiceName", gotSecond.Str())
	} else {
		require.Equal(t, "myServiceName", gotFirst.Str())
		require.Equal(t, "myLocalComponent", gotSecond.Str())
	}
}

func TestSingleJSONV1BatchToTraces(t *testing.T) {
	blob, err := os.ReadFile("./testdata/zipkin_v1_single_batch.json")
	require.NoError(t, err, "Failed to load test data")

	// This test relies on parsing int/bool to the typed span attributes
	got, err := jsonBatchToTraces(blob, true)
	require.NoError(t, err)

	compareTraces(t, tracesFromZipkinV1, got)
}

func TestMultipleJSONV1BatchesToTraces(t *testing.T) {
	blob, err := os.ReadFile("./testdata/zipkin_v1_multiple_batches.json")
	require.NoError(t, err, "Failed to load test data")

	var batches []any
	err = json.Unmarshal(blob, &batches)
	require.NoError(t, err, "Failed to load the batches")

	got := map[string]map[string]ptrace.Span{}
	for _, batch := range batches {
		jsonBatch, err := json.Marshal(batch)
		require.NoError(t, err, "Failed to marshal interface back to blob")

		// This test relies on parsing int/bool to the typed span attributes
		g, err := jsonBatchToTraces(jsonBatch, true)
		require.NoError(t, err)

		// Coalesce the nodes otherwise they will differ due to multiple
		// nodes representing same logical service
		mps := mapperTraces(t, g)
		for k, v := range mps {
			sps, ok := got[k]
			if !ok {
				sps = map[string]ptrace.Span{}
				got[k] = map[string]ptrace.Span{}
			}
			for kk, vv := range v {
				sps[kk] = vv
			}
		}
	}

	assert.Equal(t, mapperTraces(t, tracesFromZipkinV1), got)
}

func TestZipkinAnnotationsToSpanStatus(t *testing.T) {
	type test struct {
		name           string
		haveTags       []*binaryAnnotation
		wantAttributes pcommon.Map
		wantStatus     ptrace.Status
	}

	cases := []test{
		{
			name: "only otel.status_code tag",
			haveTags: []*binaryAnnotation{{
				Key:   conventions.OtelStatusCode,
				Value: "2",
			}},
			wantAttributes: pcommon.NewMap(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
		},

		{
			name: "only otel.status_description tag",
			haveTags: []*binaryAnnotation{{
				Key:   conventions.OtelStatusDescription,
				Value: "Forbidden",
			}},
			wantAttributes: pcommon.NewMap(),
			wantStatus:     ptrace.NewStatus(),
		},

		{
			name: "both otel.status_code and otel.status_description",
			haveTags: []*binaryAnnotation{
				{
					Key:   conventions.OtelStatusCode,
					Value: "2",
				},
				{
					Key:   conventions.OtelStatusDescription,
					Value: "Forbidden",
				},
			},
			wantAttributes: pcommon.NewMap(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("Forbidden")
				return ret
			}(),
		},

		{
			name: "http otel.status_code",
			haveTags: []*binaryAnnotation{
				{
					Key:   "http.status_code",
					Value: "404",
				},
				{
					Key:   "http.status_message",
					Value: "NotFound",
				},
			},
			wantAttributes: func() pcommon.Map {
				ret := pcommon.NewMap()
				ret.PutInt(conventions.AttributeHTTPStatusCode, 404)
				ret.PutStr(tracetranslator.TagHTTPStatusMsg, "NotFound")
				return ret
			}(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("NotFound")
				return ret
			}(),
		},

		{
			name: "http and oc",
			haveTags: []*binaryAnnotation{
				{
					Key:   "http.status_code",
					Value: "404",
				},
				{
					Key:   "http.status_message",
					Value: "NotFound",
				},
				{
					Key:   conventions.OtelStatusCode,
					Value: "2",
				},
				{
					Key:   conventions.OtelStatusDescription,
					Value: "Forbidden",
				},
			},
			wantAttributes: func() pcommon.Map {
				ret := pcommon.NewMap()
				ret.PutInt(conventions.AttributeHTTPStatusCode, 404)
				ret.PutStr(tracetranslator.TagHTTPStatusMsg, "NotFound")
				return ret
			}(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("Forbidden")
				return ret
			}(),
		},

		{
			name: "http and only oc code",
			haveTags: []*binaryAnnotation{
				{
					Key:   "http.status_code",
					Value: "404",
				},
				{
					Key:   "http.status_message",
					Value: "NotFound",
				},
				{
					Key:   conventions.OtelStatusCode,
					Value: "2",
				},
			},
			wantAttributes: func() pcommon.Map {
				ret := pcommon.NewMap()
				ret.PutInt(conventions.AttributeHTTPStatusCode, 404)
				ret.PutStr(tracetranslator.TagHTTPStatusMsg, "NotFound")
				return ret
			}(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
		},

		{
			name: "http and only oc message",
			haveTags: []*binaryAnnotation{
				{
					Key:   "http.status_code",
					Value: "404",
				},
				{
					Key:   "http.status_message",
					Value: "NotFound",
				},
				{
					Key:   conventions.OtelStatusDescription,
					Value: "Forbidden",
				},
			},
			wantAttributes: func() pcommon.Map {
				ret := pcommon.NewMap()
				ret.PutInt(conventions.AttributeHTTPStatusCode, 404)
				ret.PutStr(tracetranslator.TagHTTPStatusMsg, "NotFound")
				return ret
			}(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("NotFound")
				return ret
			}(),
		},

		{
			name: "census tags",
			haveTags: []*binaryAnnotation{
				{
					Key:   "census.status_code",
					Value: "10",
				},
				{
					Key:   "census.status_description",
					Value: "RPCError",
				},
			},
			wantAttributes: pcommon.NewMap(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("RPCError")
				return ret
			}(),
		},

		{
			name: "census tags priority over others",
			haveTags: []*binaryAnnotation{
				{
					Key:   "census.status_code",
					Value: "10",
				},
				{
					Key:   "census.status_description",
					Value: "RPCError",
				},
				{
					Key:   "http.status_code",
					Value: "404",
				},
				{
					Key:   "http.status_message",
					Value: "NotFound",
				},
				{
					Key:   conventions.OtelStatusDescription,
					Value: "Forbidden",
				},
				{
					Key:   conventions.OtelStatusCode,
					Value: "7",
				},
			},
			wantAttributes: func() pcommon.Map {
				ret := pcommon.NewMap()
				ret.PutInt(conventions.AttributeHTTPStatusCode, 404)
				ret.PutStr(tracetranslator.TagHTTPStatusMsg, "NotFound")
				return ret
			}(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("RPCError")
				return ret
			}(),
		},
	}

	fakeTraceID := "00000000000000010000000000000002"
	fakeSpanID := "0000000000000001"

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			zSpans := []*jsonSpan{{
				ID:                fakeSpanID,
				TraceID:           fakeTraceID,
				BinaryAnnotations: c.haveTags,
				Timestamp:         1,
			}}
			zBytes, err := json.Marshal(zSpans)
			require.NoError(t, err)

			// This test relies on parsing int/bool to the typed span attributes
			td, err := jsonBatchToTraces(zBytes, true)
			require.NoError(t, err)
			gs := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			require.Equal(t, c.wantAttributes.AsRaw(), gs.Attributes().AsRaw(), "Unsuccessful conversion %d", i)
			require.Equal(t, c.wantStatus, gs.Status(), "Unsuccessful conversion %d", i)
		})
	}
}

func TestSpanWithoutTimestampGetsTag(t *testing.T) {
	fakeTraceID := "00000000000000010000000000000002"
	fakeSpanID := "0000000000000001"
	zSpans := []*jsonSpan{
		{
			ID:        fakeSpanID,
			TraceID:   fakeTraceID,
			Timestamp: 0, // no timestamp field
		},
	}
	zBytes, err := json.Marshal(zSpans)
	require.NoError(t, err)

	testStart := time.Now()

	td, err := jsonBatchToTraces(zBytes, false)
	require.NoError(t, err)

	gs := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.NotZero(t, gs.StartTimestamp())
	assert.NotZero(t, gs.EndTimestamp())

	assert.GreaterOrEqual(t, gs.StartTimestamp().AsTime().Sub(testStart), time.Duration(0))

	wantAttributes := pcommon.NewMap()
	wantAttributes.PutBool(zipkin.StartTimeAbsent, true)
	assert.Equal(t, wantAttributes, gs.Attributes())
}

func TestSpanWithTimestamp(t *testing.T) {
	timestampMicroseconds := int64(1667492727795000)
	timestamp := pcommon.NewTimestampFromTime(time.UnixMicro(timestampMicroseconds))

	fakeTraceID := "00000000000000010000000000000002"
	fakeSpanID := "0000000000000001"
	zSpans := []*jsonSpan{
		{
			ID:        fakeSpanID,
			TraceID:   fakeTraceID,
			Timestamp: timestampMicroseconds,
		},
	}
	zBytes, err := json.Marshal(zSpans)
	require.NoError(t, err)

	td, err := jsonBatchToTraces(zBytes, false)
	require.NoError(t, err)

	gs := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, timestamp, gs.StartTimestamp())
}

func TestJSONHTTPToStatusCode(t *testing.T) {
	fakeTraceID := "00000000000000010000000000000002"
	fakeSpanID := "0000000000000001"
	for i := int32(100); i <= 600; i++ {
		wantStatus := statusCodeFromHTTP(i)
		zBytes, err := json.Marshal([]*jsonSpan{{
			ID:      fakeSpanID,
			TraceID: fakeTraceID,
			BinaryAnnotations: []*binaryAnnotation{
				{
					Key:   "http.status_code",
					Value: strconv.Itoa(int(i)),
				},
			},
		}})
		require.NoError(t, err)
		td, err := jsonBatchToTraces(zBytes, false)
		require.NoError(t, err)
		gs := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		require.Equal(t, wantStatus, gs.Status().Code(), "Unsuccessful conversion %d", i)
	}
}

// tracesFromZipkinV1 has the "pdata" Traces used in the test.
var tracesFromZipkinV1 = func() ptrace.Traces {
	td := ptrace.NewTraces()
	rm := td.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().PutStr(conventions.AttributeServiceName, "front-proxy")
	rm.Resource().Attributes().PutStr("ipv4", "172.31.0.2")

	span := rm.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetSpanID([8]byte{0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetName("checkAvailability")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 446743000)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 459699000)))

	rm = td.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service1")
	rm.Resource().Attributes().PutStr("ipv4", "172.31.0.4")

	span = rm.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetSpanID([8]byte{0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetName("checkAvailability")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 448081000)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 460102000)))
	ev := span.Events().AppendEmpty()
	ev.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 450000000)))
	ev.SetName("custom time event")

	span = rm.ScopeSpans().At(0).Spans().AppendEmpty()
	span.SetTraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetSpanID([8]byte{0xf9, 0xeb, 0xb6, 0xe6, 0x48, 0x80, 0x61, 0x2a})
	span.SetParentSpanID([8]byte{0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetName("checkStock")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 453923000)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 457663000)))

	rm = td.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service2")
	rm.Resource().Attributes().PutStr("ipv4", "172.31.0.7")
	span = rm.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetSpanID([8]byte{0xf9, 0xeb, 0xb6, 0xe6, 0x48, 0x80, 0x61, 0x2a})
	span.SetParentSpanID([8]byte{0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetName("checkStock")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 454487000)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 457320000)))
	//nolint:errcheck
	span.Attributes().FromRaw(map[string]any{
		"http.status_code": 200,
		"http.url":         "http://localhost:9000/trace/2",
		"success":          true,
		"processed":        1.5,
	})

	rm = td.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().PutStr(conventions.AttributeServiceName, "unknown-service")
	span = rm.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetSpanID([8]byte{0xfe, 0x35, 0x1a, 0x05, 0x3f, 0xbc, 0xac, 0x1f})
	span.SetParentSpanID([8]byte{0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8})
	span.SetName("checkStock")
	span.SetKind(ptrace.SpanKindUnspecified)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 453923000)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(1544805927, 457663000)))

	return td
}()

func TestSpanKindTranslation(t *testing.T) {
	tests := []struct {
		zipkinV1Kind   string
		zipkinV2Kind   zipkinmodel.Kind
		kind           ptrace.SpanKind
		jaegerSpanKind string
	}{
		{
			zipkinV1Kind:   "cr",
			zipkinV2Kind:   zipkinmodel.Client,
			kind:           ptrace.SpanKindClient,
			jaegerSpanKind: "client",
		},

		{
			zipkinV1Kind:   "sr",
			zipkinV2Kind:   zipkinmodel.Server,
			kind:           ptrace.SpanKindServer,
			jaegerSpanKind: "server",
		},

		{
			zipkinV1Kind:   "ms",
			zipkinV2Kind:   zipkinmodel.Producer,
			kind:           ptrace.SpanKindProducer,
			jaegerSpanKind: "producer",
		},

		{
			zipkinV1Kind:   "mr",
			zipkinV2Kind:   zipkinmodel.Consumer,
			kind:           ptrace.SpanKindConsumer,
			jaegerSpanKind: "consumer",
		},
	}

	for _, test := range tests {
		t.Run(test.zipkinV1Kind, func(t *testing.T) {
			// Create Zipkin V1 span.
			zSpan := &jsonSpan{
				TraceID: "1234567890123456",
				ID:      "0123456789123456",
				Annotations: []*annotation{
					{Value: test.zipkinV1Kind}, // note that only first annotation matters.
					{Value: "cr"},              // this will have no effect.
				},
			}

			// Translate to SpanAndEndpoint and verify that span kind is correctly translated.
			sae, err := jsonToSpanAndEndpoint(zSpan, false)
			assert.NoError(t, err)
			assert.Equal(t, test.kind, sae.span.Kind())
			assert.NotNil(t, sae.endpoint)
		})
	}
}

func TestZipkinV1ToSpanAnsEndpointInvalidTraceId(t *testing.T) {
	zSpan := &jsonSpan{
		TraceID: "abc",
		ID:      "0123456789123456",
		Annotations: []*annotation{
			{Value: "cr"},
		},
	}
	_, err := jsonToSpanAndEndpoint(zSpan, false)
	assert.EqualError(t, err, "zipkinV1 span traceId: hex traceId span has wrong length (expected 16 or 32)")
}

func TestZipkinV1ToSpanAnsEndpointInvalidSpanId(t *testing.T) {
	zSpan := &jsonSpan{
		TraceID: "1234567890123456",
		ID:      "abc",
		Annotations: []*annotation{
			{Value: "cr"},
		},
	}
	_, err := jsonToSpanAndEndpoint(zSpan, false)
	assert.EqualError(t, err, "zipkinV1 span id: hex Id has wrong length (expected 16)")
}
