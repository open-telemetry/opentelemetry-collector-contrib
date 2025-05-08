// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vmsgp "github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator/header"
)

var data = [2]any{
	0: []string{
		0:  "baggage",
		1:  "item",
		2:  "elasticsearch.version",
		3:  "7.0",
		4:  "my-name",
		5:  "numeric_attribute",
		6:  "my-service",
		7:  "my-resource",
		8:  "_sampling_priority_v1",
		9:  "value whatever",
		10: "sql",
		11: "service.name",
		12: "1.0.1",
		13: "version",
		14: "_dd.span_links",
		15: `[{"attributes":{"attr1":"val1","attr2":"val2"},"span_id":"70666bf9dee4a3fe","trace_id":"0eacdb57bebc935038bf5b4802ccabd5","tracestate":"dd=k:v"}]`,
	},
	1: [][][12]any{
		{
			{
				6,
				4,
				7,
				uint64(12345678901234561234),
				uint64(2),
				uint64(3),
				int64(123),
				int64(456),
				1,
				map[any]any{
					8:  9,
					0:  1,
					2:  3,
					11: 6,
					13: 12,
					14: 15,
				},
				map[any]float64{
					5: 1.2,
					8: 1,
				},
				10,
			},
		},
	},
}

func getTraces(t *testing.T) (traces pb.Traces) {
	payload, err := vmsgp.Marshal(&data)
	assert.NoError(t, err)
	if err2 := traces.UnmarshalMsgDictionary(payload); err2 != nil {
		t.Fatal(err)
	}
	return traces
}

func TestTracePayloadV05Unmarshalling(t *testing.T) {
	var traces pb.Traces

	payload, err := vmsgp.Marshal(&data)
	assert.NoError(t, err)

	require.NoError(t, traces.UnmarshalMsgDictionary(payload), "Must not error when marshaling content")
	req, _ := http.NewRequest(http.MethodPost, "/v0.5/traces", io.NopCloser(bytes.NewReader(payload)))

	tracePayloads, _ := HandleTracesPayload(req)
	assert.Len(t, tracePayloads, 1, "Expected one translated payload")
	tracePayload := tracePayloads[0]
	translated, _ := ToTraces(zap.NewNop(), tracePayload, req, nil)
	assert.Equal(t, 1, translated.SpanCount(), "Span Count wrong")
	span := translated.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.NotNil(t, span)
	assert.Equal(t, "my-name", span.Name())
	assert.Equal(t, 10, span.Attributes().Len(), "missing attributes")
	value, exists := span.Attributes().Get("service.name")
	assert.True(t, exists, "service.name missing")
	assert.Equal(t, "my-service", value.AsString(), "service.name attribute value incorrect")
	serviceVersionValue, _ := span.Attributes().Get("service.version")
	assert.Equal(t, "1.0.1", serviceVersionValue.AsString())
	spanResource, _ := span.Attributes().Get("dd.span.Resource")
	assert.Equal(t, "my-resource", spanResource.Str())
	spanResource1, _ := span.Attributes().Get("sampling.priority")
	assert.Equal(t, fmt.Sprintf("%f", 1.0), spanResource1.Str())
	numericAttributeValue, _ := span.Attributes().Get("numeric_attribute")
	numericAttributeFloat, _ := strconv.ParseFloat(numericAttributeValue.AsString(), 64)
	assert.Equal(t, 1.2, numericAttributeFloat)

	spanLink := span.Links().At(0)
	assert.Equal(t, "70666bf9dee4a3fe", spanLink.SpanID().String())
	assert.Equal(t, "0eacdb57bebc935038bf5b4802ccabd5", spanLink.TraceID().String())
	assert.Equal(t, "dd=k:v", spanLink.TraceState().AsRaw())
	spanLinkAttrVal, _ := spanLink.Attributes().Get("attr1")
	assert.Equal(t, "val1", spanLinkAttrVal.Str())
}

func TestTracePayloadV07Unmarshalling(t *testing.T) {
	traces := getTraces(t)
	apiPayload := pb.TracerPayload{
		LanguageName:    "1",
		LanguageVersion: "1",
		Chunks:          traceChunksFromTraces(traces),
		TracerVersion:   "1",
	}
	var reqBytes []byte
	bytez, _ := apiPayload.MarshalMsg(reqBytes)
	req, _ := http.NewRequest(http.MethodPost, "/v0.7/traces", io.NopCloser(bytes.NewReader(bytez)))

	translatedPayloads, _ := HandleTracesPayload(req)
	assert.Len(t, translatedPayloads, 1, "Expected one translated payload")
	translated := translatedPayloads[0]
	span := translated.GetChunks()[0].GetSpans()[0]
	assert.NotNil(t, span)
	assert.Len(t, span.GetMeta(), 6, "missing attributes")
	value, exists := span.GetMeta()["service.name"]
	assert.True(t, exists, "service.name missing")
	assert.Equal(t, "my-service", value, "service.name attribute value incorrect")
	assert.Equal(t, "my-name", span.GetName())
	assert.Equal(t, "my-resource", span.GetResource())
}

func BenchmarkTranslatorv05(b *testing.B) {
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		TestTracePayloadV05Unmarshalling(&testing.T{})
	}
	b.StopTimer()
}

func BenchmarkTranslatorv07(b *testing.B) {
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		TestTracePayloadV07Unmarshalling(&testing.T{})
	}
	b.StopTimer()
}

func TestTracePayloadApiV02Unmarshalling(t *testing.T) {
	traces := getTraces(t)
	agentPayload := agentPayloadFromTraces(&traces)

	bytez, _ := proto.Marshal(&agentPayload)
	req, _ := http.NewRequest(http.MethodPost, "/api/v0.2/traces", io.NopCloser(bytes.NewReader(bytez)))

	translatedPayloads, _ := HandleTracesPayload(req)
	assert.Len(t, translatedPayloads, 2, "Expected two translated payload")
	for _, translated := range translatedPayloads {
		assert.NotNil(t, translated)
		assert.Len(t, translated.Chunks, 1)
		assert.Len(t, translated.Chunks[0].Spans, 1)
		span := translated.Chunks[0].Spans[0]

		assert.NotNil(t, span)
		assert.Len(t, span.Meta, 6, "missing attributes")
		assert.Equal(t, "my-service", span.Meta["service.name"])
		assert.Equal(t, "my-name", span.Name)
		assert.Equal(t, "my-resource", span.Resource)
	}
}

func agentPayloadFromTraces(traces *pb.Traces) (agentPayload pb.AgentPayload) {
	numberOfTraces := 2
	var tracerPayloads []*pb.TracerPayload
	for i := 0; i < numberOfTraces; i++ {
		payload := &pb.TracerPayload{
			LanguageName:    strconv.Itoa(i),
			LanguageVersion: strconv.Itoa(i),
			ContainerID:     strconv.Itoa(i),
			Chunks:          traceChunksFromTraces(*traces),
			TracerVersion:   strconv.Itoa(i),
		}
		tracerPayloads = append(tracerPayloads, payload)
	}

	return pb.AgentPayload{
		TracerPayloads: tracerPayloads,
	}
}

func TestUpsertHeadersAttributes(t *testing.T) {
	// Test case 1: Datadog-Meta-Tracer-Version is present in headers
	req1, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	req1.Header.Set(header.TracerVersion, "1.2.3")
	attrs1 := pcommon.NewMap()
	upsertHeadersAttributes(req1, attrs1)
	val, ok := attrs1.Get(semconv.AttributeTelemetrySDKVersion)
	assert.True(t, ok)
	assert.Equal(t, "Datadog-1.2.3", val.Str())

	// Test case 2: Datadog-Meta-Lang is present in headers with ".NET"
	req2, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	req2.Header.Set(header.Lang, ".NET")
	attrs2 := pcommon.NewMap()
	upsertHeadersAttributes(req2, attrs2)
	val, ok = attrs2.Get(semconv.AttributeTelemetrySDKLanguage)
	assert.True(t, ok)
	assert.Equal(t, "dotnet", val.Str())
}

func TestToTraces64to128bits(t *testing.T) {
	// Test that we properly reconstruct a 128 bits TraceID from Datadog spans.
	// This is necessary when a datadog instrumented service has been called by an OTel instrumented one as Datadog will
	// split the TraceID into two different values:
	// * TraceID holds the lower 64 bits
	// * `_dd.p.tid` holds the upper 64 bits
	expectedTraceID128 := "f233b7e1421e8bde1d99f09757cf199d"
	expectedTraceID64 := "00000000000000001d99f09757cf199d"
	spanIDParentOtel, _ := strconv.ParseUint("6b953724b399048a", 16, 32)
	spanIDParentDD, _ := strconv.ParseUint("039f8ec65ed09993", 16, 32)
	spanIDChildDD, _ := strconv.ParseUint("5ab19b8ebe922796", 16, 32)

	spans := []pb.Span{
		{
			TraceID:  2133000431340558749,
			SpanID:   spanIDParentDD,
			ParentID: spanIDParentOtel,
			Meta: map[string]string{
				"_dd.p.tid": "f233b7e1421e8bde",
			},
		},
		{
			TraceID:  2133000431340558749,
			SpanID:   spanIDChildDD,
			ParentID: spanIDParentDD,
		},
	}
	payload := &pb.TracerPayload{
		Chunks: traceChunksFromSpans(spans),
	}

	req := &http.Request{
		Header: http.Header{},
	}
	req.Header.Set(header.Lang, "go")

	// Test 1: We reconstructed the 128 bits trace id on both spans
	cache, _ := simplelru.NewLRU[uint64, pcommon.TraceID](2, func(_ uint64, _ pcommon.TraceID) {})

	traces, _ := ToTraces(zap.NewNop(), payload, req, cache)
	assert.Equal(t, 2, traces.SpanCount(), "Expected 2 spans")

	for _, rs := range traces.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				assert.Equal(t, expectedTraceID128, span.TraceID().String())
			}
		}
	}

	// Test 2: TraceID is reconstructed only with the lower 64 bits (previous behavior)
	traces, _ = ToTraces(zap.NewNop(), payload, req, nil)
	assert.Equal(t, 2, traces.SpanCount(), "Expected 2 spans")

	for _, rs := range traces.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				assert.Equal(t, expectedTraceID64, span.TraceID().String())
			}
		}
	}
}
