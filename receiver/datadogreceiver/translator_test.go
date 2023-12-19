// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vmsgp "github.com/vmihailenco/msgpack/v4"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"google.golang.org/protobuf/proto"
)

var data = [2]any{
	0: []string{
		0:  "baggage",
		1:  "item",
		2:  "elasticsearch.version",
		3:  "7.0",
		4:  "my-name",
		5:  "X",
		6:  "my-service",
		7:  "my-resource",
		8:  "_dd.sampling_rate_whatever",
		9:  "value whatever",
		10: "sql",
		11: "service.name",
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
				},
				map[any]float64{
					5: 1.2,
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

	translated := toTraces(&pb.TracerPayload{
		LanguageName:    req.Header.Get("Datadog-Meta-Lang"),
		LanguageVersion: req.Header.Get("Datadog-Meta-Lang-Version"),
		TracerVersion:   req.Header.Get("Datadog-Meta-Tracer-Version"),
		Chunks:          traceChunksFromTraces(traces),
	}, req)
	assert.Equal(t, 1, translated.SpanCount(), "Span Count wrong")
	span := translated.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.NotNil(t, span)
	assert.Equal(t, 7, span.Attributes().Len(), "missing attributes")
	value, exists := span.Attributes().Get("service.name")
	assert.True(t, exists, "service.name missing")
	assert.Equal(t, "my-service", value.AsString(), "service.name attribute value incorrect")
	assert.Equal(t, "my-name", span.Name())
	spanResource, _ := span.Attributes().Get("dd.span.Resource")
	assert.Equal(t, "my-resource", spanResource.Str())
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

	translatedPayloads, _ := handlePayload(req)
	assert.Equal(t, len(translatedPayloads), 1, "Expected one translated payload")
	translated := translatedPayloads[0]
	span := translated.GetChunks()[0].GetSpans()[0]
	assert.NotNil(t, span)
	assert.Equal(t, 4, len(span.GetMeta()), "missing attributes")
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

	translatedPayloads, _ := handlePayload(req)
	assert.Equal(t, len(translatedPayloads), 2, "Expected two translated payload")
	for _, translated := range translatedPayloads {
		assert.NotNil(t, translated)
		assert.Equal(t, 1, len(translated.Chunks))
		assert.Equal(t, 1, len(translated.Chunks[0].Spans))
		span := translated.Chunks[0].Spans[0]

		assert.NotNil(t, span)
		assert.Equal(t, 4, len(span.Meta), "missing attributes")
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
			LanguageName:    fmt.Sprintf("%d", i),
			LanguageVersion: fmt.Sprintf("%d", i),
			Chunks:          traceChunksFromTraces(*traces),
			TracerVersion:   fmt.Sprintf("%d", i),
		}
		tracerPayloads = append(tracerPayloads, payload)
	}

	return pb.AgentPayload{
		TracerPayloads: tracerPayloads,
	}
}

func TestUpsertHeadersAttributes(t *testing.T) {
	// Test case 1: Datadog-Meta-Tracer-Version is present in headers
	req1, _ := http.NewRequest("GET", "http://example.com", nil)
	req1.Header.Set("Datadog-Meta-Tracer-Version", "1.2.3")
	attrs1 := pcommon.NewMap()
	upsertHeadersAttributes(req1, attrs1)
	val, ok := attrs1.Get(semconv.AttributeTelemetrySDKVersion)
	assert.True(t, ok)
	assert.Equal(t, "Datadog-1.2.3", val.Str())

	// Test case 2: Datadog-Meta-Lang is present in headers with ".NET"
	req2, _ := http.NewRequest("GET", "http://example.com", nil)
	req2.Header.Set("Datadog-Meta-Lang", ".NET")
	attrs2 := pcommon.NewMap()
	upsertHeadersAttributes(req2, attrs2)
	val, ok = attrs2.Get(semconv.AttributeTelemetrySDKLanguage)
	assert.True(t, ok)
	assert.Equal(t, "dotnet", val.Str())
}
