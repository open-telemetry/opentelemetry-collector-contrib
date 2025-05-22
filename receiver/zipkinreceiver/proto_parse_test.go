// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinreceiver

import (
	"net/http"
	"testing"
	"time"

	"github.com/openzipkin/zipkin-go/proto/zipkin_proto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.12.0"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
)

func TestConvertSpansToTraceSpans_protobuf(t *testing.T) {
	// TODO: (@odeke-em) examine the entire codepath that time goes through
	//  in Zipkin-Go to ensure that all rounding matches. Otherwise
	// for now we need to round Annotation times to seconds for comparison.
	cmpTimestamp := func(t time.Time) time.Time {
		return t.Round(time.Second)
	}

	now := cmpTimestamp(time.Date(2018, 10, 31, 19, 43, 35, 789, time.UTC))
	minus10hr5ms := cmpTimestamp(now.Add(-(10*time.Hour + 5*time.Millisecond)))

	// 1. Generate some spans then serialize them with protobuf
	payloadFromWild := &zipkin_proto3.ListOfSpans{
		Spans: []*zipkin_proto3.Span{
			{
				TraceId:   []byte{0x7F, 0x6F, 0x5F, 0x4F, 0x3F, 0x2F, 0x1F, 0x0F, 0xF7, 0xF6, 0xF5, 0xF4, 0xF3, 0xF2, 0xF1, 0xF0},
				Id:        []byte{0xF7, 0xF6, 0xF5, 0xF4, 0xF3, 0xF2, 0xF1, 0xF0},
				ParentId:  []byte{0xF7, 0xF6, 0xF5, 0xF4, 0xF3, 0xF2, 0xF1, 0xF0},
				Name:      "ProtoSpan1",
				Kind:      zipkin_proto3.Span_CONSUMER,
				Timestamp: uint64(now.UnixNano() / 1e3),
				Duration:  12e6, // 12 seconds
				LocalEndpoint: &zipkin_proto3.Endpoint{
					ServiceName: "svc-1",
					Ipv4:        []byte{0xC0, 0xA8, 0x00, 0x01},
					Port:        8009,
				},
				RemoteEndpoint: &zipkin_proto3.Endpoint{
					ServiceName: "memcached",
					Ipv6:        []byte{0xFE, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x14, 0x53, 0xa7, 0x7c, 0xda, 0x4d, 0xd2, 0x1b},
					Port:        11211,
				},
			},
			{
				TraceId:   []byte{0x7A, 0x6A, 0x5A, 0x4A, 0x3A, 0x2A, 0x1A, 0x0A, 0xC7, 0xC6, 0xC5, 0xC4, 0xC3, 0xC2, 0xC1, 0xC0},
				Id:        []byte{0x67, 0x66, 0x65, 0x64, 0x63, 0x62, 0x61, 0x60},
				ParentId:  []byte{0x17, 0x16, 0x15, 0x14, 0x13, 0x12, 0x11, 0x10},
				Name:      "CacheWarmUp",
				Kind:      zipkin_proto3.Span_PRODUCER,
				Timestamp: uint64(minus10hr5ms.UnixNano() / 1e3),
				Duration:  7e6, // 7 seconds
				LocalEndpoint: &zipkin_proto3.Endpoint{
					ServiceName: "search",
					Ipv4:        []byte{0x0A, 0x00, 0x00, 0x0D},
					Port:        8009,
				},
				RemoteEndpoint: &zipkin_proto3.Endpoint{
					ServiceName: "redis",
					Ipv6:        []byte{0xFE, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x14, 0x53, 0xa7, 0x7c, 0xda, 0x4d, 0xd2, 0x1b},
					Port:        6379,
				},
				Annotations: []*zipkin_proto3.Annotation{
					{
						Timestamp: uint64(minus10hr5ms.UnixNano() / 1e3),
						Value:     "DB reset",
					},
					{
						Timestamp: uint64(minus10hr5ms.UnixNano() / 1e3),
						Value:     "GC Cycle 39",
					},
				},
			},
		},
	}

	// 2. Serialize it
	protoBlob, err := proto.Marshal(payloadFromWild)
	require.NoError(t, err, "Failed to protobuf serialize payload: %v", err)
	zi := newTestZipkinReceiver()
	hdr := make(http.Header)
	hdr.Set("Content-Type", "application/x-protobuf")

	// 3. Get that payload converted to OpenCensus proto spans.
	reqs, err := zi.v2ToTraceSpans(protoBlob, hdr)
	require.NoError(t, err, "Failed to parse convert Zipkin spans in Protobuf to Trace spans: %v", err)
	require.Equal(t, 2, reqs.ResourceSpans().Len(), "Expecting exactly 2 requests since spans have different node/localEndpoint: %v", reqs.ResourceSpans().Len())

	want := ptrace.NewTraces()
	want.ResourceSpans().EnsureCapacity(2)

	// First span/resource
	want.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "svc-1")
	span0 := want.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span0.SetTraceID([16]byte{0x7F, 0x6F, 0x5F, 0x4F, 0x3F, 0x2F, 0x1F, 0x0F, 0xF7, 0xF6, 0xF5, 0xF4, 0xF3, 0xF2, 0xF1, 0xF0})
	span0.SetSpanID([8]byte{0xF7, 0xF6, 0xF5, 0xF4, 0xF3, 0xF2, 0xF1, 0xF0})
	span0.SetParentSpanID([8]byte{0xF7, 0xF6, 0xF5, 0xF4, 0xF3, 0xF2, 0xF1, 0xF0})
	span0.SetName("ProtoSpan1")
	span0.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	span0.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(12 * time.Second)))
	span0.Attributes().PutStr(string(conventions.NetHostIPKey), "192.168.0.1")
	span0.Attributes().PutInt(string(conventions.NetHostPortKey), 8009)
	span0.Attributes().PutStr(string(conventions.NetPeerNameKey), "memcached")
	span0.Attributes().PutStr(string(conventions.NetPeerIPKey), "fe80::1453:a77c:da4d:d21b")
	span0.Attributes().PutInt(string(conventions.NetPeerPortKey), 11211)
	span0.Attributes().PutStr(tracetranslator.TagSpanKind, string(tracetranslator.OpenTracingSpanKindConsumer))

	// Second span/resource
	want.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "search")
	span1 := want.ResourceSpans().At(1).ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID([16]byte{0x7A, 0x6A, 0x5A, 0x4A, 0x3A, 0x2A, 0x1A, 0x0A, 0xC7, 0xC6, 0xC5, 0xC4, 0xC3, 0xC2, 0xC1, 0xC0})
	span1.SetSpanID([8]byte{0x67, 0x66, 0x65, 0x64, 0x63, 0x62, 0x61, 0x60})
	span1.SetParentSpanID([8]byte{0x17, 0x16, 0x15, 0x14, 0x13, 0x12, 0x11, 0x10})
	span1.SetName("CacheWarmUp")
	span1.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-10 * time.Hour)))
	span1.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(-10 * time.Hour).Add(7 * time.Second)))
	span1.Attributes().PutStr(string(conventions.NetHostIPKey), "10.0.0.13")
	span1.Attributes().PutInt(string(conventions.NetHostPortKey), 8009)
	span1.Attributes().PutStr(string(conventions.NetPeerNameKey), "redis")
	span1.Attributes().PutStr(string(conventions.NetPeerIPKey), "fe80::1453:a77c:da4d:d21b")
	span1.Attributes().PutInt(string(conventions.NetPeerPortKey), 6379)
	span1.Attributes().PutStr(tracetranslator.TagSpanKind, string(tracetranslator.OpenTracingSpanKindProducer))
	span1.Events().EnsureCapacity(2)
	span1.Events().AppendEmpty().SetName("DB reset")
	span1.Events().At(0).SetTimestamp(pcommon.NewTimestampFromTime(now.Add(-10 * time.Hour)))
	span1.Events().AppendEmpty().SetName("GC Cycle 39")
	span1.Events().At(1).SetTimestamp(pcommon.NewTimestampFromTime(now.Add(-10 * time.Hour)))

	assert.Equal(t, want.SpanCount(), reqs.SpanCount())
	assert.Equal(t, want.ResourceSpans().Len(), reqs.ResourceSpans().Len())
	for i := 0; i < want.ResourceSpans().Len(); i++ {
		wantRS := want.ResourceSpans().At(i)
		wSvcName, ok := wantRS.Resource().Attributes().Get(string(conventions.ServiceNameKey))
		assert.True(t, ok)
		for j := 0; j < reqs.ResourceSpans().Len(); j++ {
			reqsRS := reqs.ResourceSpans().At(j)
			rSvcName, ok := reqsRS.Resource().Attributes().Get(string(conventions.ServiceNameKey))
			assert.True(t, ok)
			if rSvcName.Str() == wSvcName.Str() {
				compareResourceSpans(t, wantRS, reqsRS)
			}
		}
	}
}

func newTestZipkinReceiver() *zipkinReceiver {
	cfg := createDefaultConfig().(*Config)
	return &zipkinReceiver{
		config:                   cfg,
		jsonUnmarshaler:          zipkinv2.NewJSONTracesUnmarshaler(cfg.ParseStringTags),
		protobufUnmarshaler:      zipkinv2.NewProtobufTracesUnmarshaler(false, cfg.ParseStringTags),
		protobufDebugUnmarshaler: zipkinv2.NewProtobufTracesUnmarshaler(true, cfg.ParseStringTags),
	}
}

func compareResourceSpans(t *testing.T, wantRS ptrace.ResourceSpans, reqsRS ptrace.ResourceSpans) {
	assert.Equal(t, wantRS.ScopeSpans().Len(), reqsRS.ScopeSpans().Len())
	wantIL := wantRS.ScopeSpans().At(0)
	reqsIL := reqsRS.ScopeSpans().At(0)
	assert.Equal(t, wantIL.Spans().Len(), reqsIL.Spans().Len())
}
