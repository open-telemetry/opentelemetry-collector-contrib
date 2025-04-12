// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinv1

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/jaegertracing/jaeger-idl/thrift-gen/zipkincore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
	zipkin "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinthriftconverter"
)

// compareTraces compares got to want while ignoring order. Both are modified in place.
func compareTraces(t *testing.T, want ptrace.Traces, got ptrace.Traces) {
	require.Equal(t, mapperTraces(t, want), mapperTraces(t, got))
}

func mapperTraces(t *testing.T, td ptrace.Traces) map[string]map[string]ptrace.Span {
	ret := map[string]map[string]ptrace.Span{}
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		service, found := rs.Resource().Attributes().Get(conventions.AttributeServiceName)
		require.True(t, found)
		sps, ok := ret[service.Str()]
		if !ok {
			sps = map[string]ptrace.Span{}
			ret[service.Str()] = map[string]ptrace.Span{}
		}
		spans := rs.ScopeSpans().At(0).Spans()
		for j := 0; j < spans.Len(); j++ {
			sps[spans.At(j).Name()] = spans.At(j)
		}
	}
	return ret
}

func TestV1ThriftToTraces(t *testing.T) {
	blob, err := os.ReadFile("./testdata/zipkin_v1_thrift_single_batch.json")
	require.NoError(t, err, "Failed to load test data")

	var zSpans []*zipkincore.Span
	require.NoError(t, json.Unmarshal(blob, &zSpans), "failed to unmarshal json test file")
	thriftBytes, err := zipkin.SerializeThrift(context.TODO(), zSpans)
	require.NoError(t, err)
	td, err := thriftUnmarshaler{}.UnmarshalTraces(thriftBytes)
	require.NoError(t, err, "Failed to translate zipkinv1 thrift to OC proto")
	assert.Equal(t, 5, td.SpanCount())
}

func TestZipkinThriftFallbackToLocalComponent(t *testing.T) {
	blob, err := os.ReadFile("./testdata/zipkin_v1_thrift_local_component.json")
	require.NoError(t, err, "Failed to load test data")

	var ztSpans []*zipkincore.Span
	err = json.Unmarshal(blob, &ztSpans)
	require.NoError(t, err, "Failed to unmarshal json into zipkin v1 thrift")

	reqs, err := thriftBatchToTraces(ztSpans)
	require.NoError(t, err, "Failed to translate zipkinv1 thrift to OC proto")
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

func TestV1ThriftToOCProto(t *testing.T) {
	blob, err := os.ReadFile("./testdata/zipkin_v1_thrift_single_batch.json")
	require.NoError(t, err, "Failed to load test data")

	var ztSpans []*zipkincore.Span
	err = json.Unmarshal(blob, &ztSpans)
	require.NoError(t, err, "Failed to unmarshal json into zipkin v1 thrift")

	got, err := thriftBatchToTraces(ztSpans)
	require.NoError(t, err, "Failed to translate zipkinv1 thrift to OC proto")

	compareTraces(t, got, tracesFromZipkinV1)
}

func BenchmarkV1ThriftToOCProto(b *testing.B) {
	blob, err := os.ReadFile("./testdata/zipkin_v1_thrift_single_batch.json")
	require.NoError(b, err, "Failed to load test data")

	var ztSpans []*zipkincore.Span
	err = json.Unmarshal(blob, &ztSpans)
	require.NoError(b, err, "Failed to unmarshal json into zipkin v1 thrift")

	for n := 0; n < b.N; n++ {
		_, err = thriftBatchToTraces(ztSpans)
		require.NoError(b, err)
	}
}

func TestZipkinThriftAnnotationsToOCStatus(t *testing.T) {
	type test struct {
		name           string
		haveTags       []*zipkincore.BinaryAnnotation
		wantAttributes pcommon.Map
		wantStatus     ptrace.Status
	}

	cases := []test{
		{
			name: "too large code for OC",
			haveTags: []*zipkincore.BinaryAnnotation{{
				Key:            conventions.OtelStatusCode,
				Value:          uint64ToBytes(math.MaxInt64),
				AnnotationType: zipkincore.AnnotationType_I64,
			}},
			wantAttributes: pcommon.NewMap(),
			wantStatus:     ptrace.NewStatus(),
		},
		{
			name: "census.status_code int64",
			haveTags: []*zipkincore.BinaryAnnotation{{
				Key:            tagZipkinCensusCode,
				Value:          uint64ToBytes(5),
				AnnotationType: zipkincore.AnnotationType_I64,
			}},
			wantAttributes: pcommon.NewMap(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
		},
		{
			name: "census.status_code int32",
			haveTags: []*zipkincore.BinaryAnnotation{{
				Key:            tagZipkinCensusCode,
				Value:          uint32ToBytes(6),
				AnnotationType: zipkincore.AnnotationType_I32,
			}},
			wantAttributes: pcommon.NewMap(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
		},
		{
			name: "census.status_code int16",
			haveTags: []*zipkincore.BinaryAnnotation{{
				Key:            tagZipkinCensusCode,
				Value:          uint16ToBytes(7),
				AnnotationType: zipkincore.AnnotationType_I16,
			}},
			wantAttributes: pcommon.NewMap(),
			wantStatus: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
		},
		{
			name: "only status.message tag",
			haveTags: []*zipkincore.BinaryAnnotation{{
				Key:            conventions.OtelStatusDescription,
				Value:          []byte("Forbidden"),
				AnnotationType: zipkincore.AnnotationType_STRING,
			}},
			wantAttributes: pcommon.NewMap(),
			wantStatus:     ptrace.NewStatus(),
		},
		{
			name: "both status.code and status.message",
			haveTags: []*zipkincore.BinaryAnnotation{
				{
					Key:            conventions.OtelStatusCode,
					Value:          uint32ToBytes(2),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            conventions.OtelStatusDescription,
					Value:          []byte("Forbidden"),
					AnnotationType: zipkincore.AnnotationType_STRING,
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
			name: "http status.code",
			haveTags: []*zipkincore.BinaryAnnotation{
				{
					Key:            "http.status_code",
					Value:          uint32ToBytes(404),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            "http.status_message",
					Value:          []byte("NotFound"),
					AnnotationType: zipkincore.AnnotationType_STRING,
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
			name: "http and otel",
			haveTags: []*zipkincore.BinaryAnnotation{
				{
					Key:            "http.status_code",
					Value:          uint32ToBytes(404),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            "http.status_message",
					Value:          []byte("NotFound"),
					AnnotationType: zipkincore.AnnotationType_STRING,
				},
				{
					Key:            conventions.OtelStatusCode,
					Value:          uint32ToBytes(2),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            conventions.OtelStatusDescription,
					Value:          []byte("Forbidden"),
					AnnotationType: zipkincore.AnnotationType_STRING,
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
			name: "http and only otel code",
			haveTags: []*zipkincore.BinaryAnnotation{
				{
					Key:            "http.status_code",
					Value:          uint32ToBytes(404),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            "http.status_message",
					Value:          []byte("NotFound"),
					AnnotationType: zipkincore.AnnotationType_STRING,
				},
				{
					Key:            conventions.OtelStatusCode,
					Value:          uint32ToBytes(2),
					AnnotationType: zipkincore.AnnotationType_I32,
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
			haveTags: []*zipkincore.BinaryAnnotation{
				{
					Key:            "http.status_code",
					Value:          uint32ToBytes(404),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            "http.status_message",
					Value:          []byte("NotFound"),
					AnnotationType: zipkincore.AnnotationType_STRING,
				},
				{
					Key:            conventions.OtelStatusDescription,
					Value:          []byte("Forbidden"),
					AnnotationType: zipkincore.AnnotationType_STRING,
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
			haveTags: []*zipkincore.BinaryAnnotation{
				{
					Key:            "census.status_code",
					Value:          uint32ToBytes(18),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            "census.status_description",
					Value:          []byte("RPCError"),
					AnnotationType: zipkincore.AnnotationType_STRING,
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
			haveTags: []*zipkincore.BinaryAnnotation{
				{
					Key:            "census.status_code",
					Value:          uint32ToBytes(18),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            "census.status_description",
					Value:          []byte("RPCError"),
					AnnotationType: zipkincore.AnnotationType_STRING,
				},
				{
					Key:            "http.status_code",
					Value:          uint32ToBytes(404),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
				{
					Key:            "http.status_message",
					Value:          []byte("NotFound"),
					AnnotationType: zipkincore.AnnotationType_STRING,
				},
				{
					Key:            conventions.OtelStatusDescription,
					Value:          []byte("Forbidden"),
					AnnotationType: zipkincore.AnnotationType_STRING,
				},
				{
					Key:            conventions.OtelStatusCode,
					Value:          uint32ToBytes(1),
					AnnotationType: zipkincore.AnnotationType_I32,
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

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			zSpans := []*zipkincore.Span{{
				ID:                1,
				TraceID:           1,
				BinaryAnnotations: c.haveTags,
			}}
			td, err := thriftBatchToTraces(zSpans)
			require.NoError(t, err)
			gs := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			require.Equal(t, c.wantAttributes.AsRaw(), gs.Attributes().AsRaw())
			require.Equal(t, c.wantStatus, gs.Status())
		})
	}
}

func TestThriftHTTPToStatusCode(t *testing.T) {
	for i := int32(100); i <= 600; i++ {
		wantStatus := statusCodeFromHTTP(i)
		td, err := thriftBatchToTraces([]*zipkincore.Span{{
			ID:      1,
			TraceID: 1,
			BinaryAnnotations: []*zipkincore.BinaryAnnotation{
				{
					Key:            "http.status_code",
					Value:          uint32ToBytes(uint32(i)),
					AnnotationType: zipkincore.AnnotationType_I32,
				},
			},
		}})
		require.NoError(t, err)
		gs := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		require.Equal(t, wantStatus, gs.Status().Code(), "Unsuccessful conversion %d", i)
	}
}

func Test_bytesInt16ToInt64(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		want    int64
		wantErr error
	}{
		{
			name:    "too short byte slice",
			bytes:   nil,
			want:    0,
			wantErr: errNotEnoughBytes,
		},
		{
			name:    "exact size byte slice",
			bytes:   []byte{0, 200},
			want:    200,
			wantErr: nil,
		},
		{
			name:    "large byte slice",
			bytes:   []byte{0, 128, 200, 200},
			want:    128,
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bytesInt16ToInt64(tt.bytes)
			require.ErrorIs(t, err, tt.wantErr)
			if got != tt.want {
				t.Errorf("bytesInt16ToInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bytesInt32ToInt64(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		want    int64
		wantErr error
	}{
		{
			name:    "too short byte slice",
			bytes:   []byte{},
			want:    0,
			wantErr: errNotEnoughBytes,
		},
		{
			name:    "exact size byte slice",
			bytes:   []byte{0, 0, 0, 202},
			want:    202,
			wantErr: nil,
		},
		{
			name:    "large byte slice",
			bytes:   []byte{0, 0, 0, 128, 0, 0, 0, 0},
			want:    128,
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bytesInt32ToInt64(tt.bytes)
			require.ErrorIs(t, err, tt.wantErr)
			if got != tt.want {
				t.Errorf("bytesInt32ToInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bytesInt64ToInt64(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		want    int64
		wantErr error
	}{
		{
			name:    "too short byte slice",
			bytes:   []byte{0, 0, 0, 0},
			want:    0,
			wantErr: errNotEnoughBytes,
		},
		{
			name:    "exact size byte slice",
			bytes:   []byte{0, 0, 0, 0, 0, 0, 0, 202},
			want:    202,
			wantErr: nil,
		},
		{
			name:    "large byte slice",
			bytes:   []byte{0, 0, 0, 0, 0, 0, 0, 128, 0, 0, 0, 0},
			want:    128,
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bytesInt64ToInt64(tt.bytes)
			require.ErrorIs(t, err, tt.wantErr)
			if got != tt.want {
				t.Errorf("bytesInt64ToInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bytesFloat64ToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		want    float64
		wantErr error
	}{
		{
			name:    "too short byte slice",
			bytes:   []byte{0, 0, 0, 0},
			want:    0,
			wantErr: errNotEnoughBytes,
		},
		{
			name:    "exact size byte slice",
			bytes:   []byte{64, 9, 33, 251, 84, 68, 45, 24},
			want:    3.141592653589793,
			wantErr: nil,
		},
		{
			name:    "large byte slice",
			bytes:   []byte{64, 9, 33, 251, 84, 68, 45, 24, 0, 0, 0, 0},
			want:    3.141592653589793,
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bytesFloat64ToFloat64(tt.bytes)
			require.ErrorIs(t, err, tt.wantErr)
			if got != tt.want {
				t.Errorf("bytesFloat64ToFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func uint64ToBytes(i uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, i)
	return b
}

func uint32ToBytes(i uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, i)
	return b
}

func uint16ToBytes(i uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, i)
	return b
}
