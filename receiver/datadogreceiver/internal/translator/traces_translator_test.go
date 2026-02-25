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
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vmsgp "github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
	for b.Loop() {
		TestTracePayloadV05Unmarshalling(&testing.T{})
	}
	b.StopTimer()
}

func BenchmarkTranslatorv07(b *testing.B) {
	for b.Loop() {
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
	for i := range numberOfTraces {
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
	req1, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req1.Header.Set(header.TracerVersion, "1.2.3")
	attrs1 := pcommon.NewMap()
	upsertHeadersAttributes(req1, attrs1)
	val, ok := attrs1.Get("telemetry.sdk.version")
	assert.True(t, ok)
	assert.Equal(t, "Datadog-1.2.3", val.Str())

	// Test case 2: Datadog-Meta-Lang is present in headers with ".NET"
	req2, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req2.Header.Set(header.Lang, ".NET")
	attrs2 := pcommon.NewMap()
	upsertHeadersAttributes(req2, attrs2)
	val, ok = attrs2.Get("telemetry.sdk.language")
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
	cache, _ := lru.NewWithEvict(2, func(_ uint64, _ pcommon.TraceID) {})

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

func TestToTracesServiceName(t *testing.T) {
	cases := []struct {
		name                string
		expectedServiceName string
		expectedSpanName    string
		spans               []pb.Span
	}{
		{
			name:                "check-base-service",
			expectedServiceName: "my-service",
			expectedSpanName:    "postgresql",
			spans: []pb.Span{
				{
					Name: "postgresql",
					Meta: map[string]string{
						"_dd.base_service": "my-service",
					},
				},
			},
		},
		{
			name:                "check-newspan-has-postprocessing",
			expectedServiceName: "my-service",
			expectedSpanName:    "POST",
			spans: []pb.Span{
				{
					Name: "servlet.request",
					Meta: map[string]string{
						"http.method":      "POST",
						"_dd.base_service": "my-service",
					},
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			payload := &pb.TracerPayload{
				Chunks: traceChunksFromSpans(tt.spans),
			}

			req := &http.Request{
				Header: http.Header{},
			}

			traces, _ := ToTraces(zap.NewNop(), payload, req, nil)
			for _, rs := range traces.ResourceSpans().All() {
				actualServiceName, _ := rs.Resource().Attributes().Get("service.name")
				assert.Equal(t, tt.expectedServiceName, actualServiceName.AsString())
				for _, ss := range rs.ScopeSpans().All() {
					for _, span := range ss.Spans().All() {
						assert.Equal(t, tt.expectedSpanName, span.Name())
					}
				}
			}
		})
	}
}

func TestToTraces(t *testing.T) {
	cases := []struct {
		name                   string
		ddSpan                 pb.Span
		expectedSpanName       string
		expectedSpanAttributes map[string]string
	}{
		{
			"db-query-summary",
			pb.Span{
				Name: "postgresql.query",
				Meta: map[string]string{
					"db.query.summary": "select customers",
					"db.statement":     "select * from customers",
					"db.operation":     "select",
					"db.instance":      "pg_customers",
					"db.sql.table":     "customers",
					"db.type":          "postgresql",
					"peer.hostname":    "localhost",
				},
			},
			"select customers",
			map[string]string{
				"db.query.summary":   "select customers",
				"db.query.text":      "select * from customers",
				"db.operation.name":  "select",
				"db.collection.name": "customers",
				"db.namespace":       "pg_customers",
			},
		},
		{
			"db-operation-instance",
			pb.Span{
				Name: "postgresql.query",
				Meta: map[string]string{
					"db.operation":  "select",
					"db.instance":   "pg_customers",
					"db.type":       "postgresql",
					"db.namespace":  "namespace",
					"peer.hostname": "localhost",
				},
			},
			"select pg_customers",
			map[string]string{},
		},
		{
			"db-operation-namespace",
			pb.Span{
				Name: "postgresql.query",
				Meta: map[string]string{
					"db.operation":  "select",
					"db.type":       "postgresql",
					"db.namespace":  "pg_customers",
					"peer.hostname": "localhost",
				},
			},
			"select pg_customers",
			map[string]string{},
		},
		{
			"db-operation-hostname",
			pb.Span{
				Name: "postgresql.query",
				Meta: map[string]string{
					"db.operation":  "select",
					"db.type":       "postgresql",
					"peer.hostname": "localhost",
				},
			},
			"select localhost",
			map[string]string{},
		},
		{
			"db-operation",
			pb.Span{
				Name: "postgresql.query",
				Meta: map[string]string{
					"db.operation": "select",
					"db.type":      "postgresql",
				},
			},
			"select",
			map[string]string{},
		},
		{
			"db-type",
			pb.Span{
				Name: "postgresql.query",
				Meta: map[string]string{
					"db.instance":   "instance",
					"db.type":       "postgresql",
					"db.namespace":  "namespace",
					"peer.hostname": "localhost",
				},
			},
			"postgresql",
			map[string]string{},
		},
		{
			"db-redis",
			pb.Span{
				Name: "redis.query",
				Meta: map[string]string{
					"db.type": "redis",
				},
			},
			"redis",
			map[string]string{},
		},
		{
			"internal-spring-handler",
			pb.Span{
				Name:     "spring.handler",
				Resource: "ShippingController.shipOrder",
			},
			"ShippingController.shipOrder",
			map[string]string{},
		},
		{
			"http-request-request-with-route",
			pb.Span{
				Name: "http.request",
				Meta: map[string]string{
					"http.method": "POST",
					"http.route":  "/api/order",
				},
			},
			"POST /api/order",
			map[string]string{
				"http.request.method": "POST",
				"http.route":          "/api/order",
			},
		},
		{
			"web-request-request-withut-route",
			pb.Span{
				Name: "web.request",
				Meta: map[string]string{
					"http.method": "POST",
				},
			},
			"POST",
			map[string]string{
				"http.request.method": "POST",
			},
		},
		{
			"http-servlet-request-no-route",
			pb.Span{
				Name: "servlet.request",
				Meta: map[string]string{
					"http.method": "POST",
				},
			},
			"POST",
			map[string]string{
				"http.request.method": "POST",
			},
		},
		{
			"http-servlet-request-with-route",
			pb.Span{
				Name: "servlet.request",
				Meta: map[string]string{
					"http.method": "POST",
					"http.route":  "/route",
				},
			},
			"POST /route",
			map[string]string{
				"http.request.method": "POST",
				"http.route":          "/route",
			},
		},
		{
			"rpc.client-ok-with-method-and-service",
			pb.Span{
				Name: "grpc.client",
				Meta: map[string]string{
					"rpc.grpc.full_method": "/mydomain.MyDomainService/MyMethod",
					"grpc.code":            "OK",
					"rpc.system":           "grpc",
				},
				Error: 0,
			},
			"mydomain.MyDomainService/MyMethod",
			map[string]string{
				"rpc.service":          "mydomain.MyDomainService",
				"rpc.method":           "MyMethod",
				"rpc.grpc.status_code": "0",
			},
		},
		{
			"rpc.client-ok-dd-trace-java-style",
			pb.Span{
				Name:     "grpc.client",
				Resource: "/mydomain.MyDomainService/MyMethod",
				Meta: map[string]string{
					"grpc.code": "OK",
				},
				Error: 0,
			},
			"mydomain.MyDomainService/MyMethod",
			map[string]string{
				"rpc.service":          "mydomain.MyDomainService",
				"rpc.method":           "MyMethod",
				"rpc.grpc.status_code": "0",
				"rpc.system":           "grpc",
			},
		},
		{
			"rpc.client-error-with-method-and-service",
			pb.Span{
				Name: "grpc.client",
				Meta: map[string]string{
					"rpc.grpc.full_method": "/mydomain.MyDomainService/MyMethod",
					"grpc.code":            "UNKNOWN",
					"rpc.system":           "grpc",
				},
				Error: 2,
			},
			"mydomain.MyDomainService/MyMethod",
			map[string]string{
				"rpc.service":          "mydomain.MyDomainService",
				"rpc.method":           "MyMethod",
				"rpc.grpc.status_code": "2",
			},
		},
		{
			"rpc.client-ok-with-unexpected-full-method-format-missing-method",
			pb.Span{
				Name: "grpc.client",
				Meta: map[string]string{
					"rpc.grpc.full_method": "/mydomain.MyDomainService/",
					"grpc.code":            "OK",
					"rpc.system":           "grpc",
				},
				Error: 0,
			},
			"mydomain.MyDomainService",
			map[string]string{
				"rpc.service":          "mydomain.MyDomainService",
				"rpc.grpc.status_code": "0",
			},
		},
		{
			"rpc.client-ok-with-unexpected-full-method-format-missing-slash",
			pb.Span{
				Name: "grpc.client",
				Meta: map[string]string{
					"rpc.grpc.full_method": "mydomain.MyDomainService/MyMethod",
					"grpc.code":            "OK",
					"rpc.system":           "grpc",
				},
				Error: 0,
			},
			"mydomain.MyDomainService/MyMethod",
			map[string]string{
				"rpc.service":          "mydomain.MyDomainService",
				"rpc.method":           "MyMethod",
				"rpc.grpc.status_code": "0",
			},
		},
		{
			"rpc.server-ok-with-method-and-service",
			pb.Span{
				Name: "grpc.server",
				Meta: map[string]string{
					"rpc.grpc.full_method": "/mydomain.MyDomainService/MyMethod",
					"grpc.code":            "OK",
					"rpc.system":           "grpc",
				},
				Error: 0,
			},
			"mydomain.MyDomainService/MyMethod",
			map[string]string{
				"rpc.service":          "mydomain.MyDomainService",
				"rpc.method":           "MyMethod",
				"rpc.grpc.status_code": "0",
			},
		},
		{
			"aws-s3-headObject",
			pb.Span{
				Name: "aws.request",
				Meta: map[string]string{
					"aws.service":             "S3",
					"aws.operation":           "headObject",
					"aws.s3.bucket_name":      "my-bucket",
					"aws.response.request_id": "1234567890",
				},
			},
			"S3/headObject",
			map[string]string{
				"rpc.system":     "aws-api",
				"rpc.service":    "S3",
				"rpc.method":     "headObject",
				"aws.s3.bucket":  "my-bucket",
				"aws.request_id": "1234567890",
			},
		},
	}

	//nolint:govet
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			payload := &pb.TracerPayload{
				LanguageName:    "go",
				LanguageVersion: "1.20",
				TracerVersion:   "v1",
				ContainerID:     "container-123",
				Chunks: []*pb.TraceChunk{
					{
						Priority: 0,
						Spans:    []*pb.Span{&tt.ddSpan},
					},
				},
			}
			logger, _ := zap.NewDevelopment()
			traceIDCache, _ := lru.New[uint64, pcommon.TraceID](100)
			req, _ := http.NewRequest(http.MethodPost, "/v0.5/traces", http.NoBody)

			got, err := ToTraces(logger, payload, req, traceIDCache)
			assert.NoError(t, err)
			assert.Equal(t, 1, got.SpanCount())
			gotSpan := got.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, tt.expectedSpanName, gotSpan.Name())
			for k, want := range tt.expectedSpanAttributes {
				val, ok := gotSpan.Attributes().Get(k)
				assert.True(t, ok, "attribute %s missing", k)
				assert.Equal(t, want, val.AsString(), "attribute %s value mismatch", k)
			}
		})
	}
}

func TestToTracesServerAddress(t *testing.T) {
	cases := []struct {
		name                  string
		expectedServerAddress string
		spans                 []pb.Span
	}{
		{
			name:                  "client-server-address-already-set",
			expectedServerAddress: "serverAddress",
			spans: []pb.Span{
				{
					Name: "span",
					Meta: map[string]string{
						"span.kind":      "client",
						"server.address": "serverAddress",
						"peer.hostname":  "peerHostname",
					},
				},
			},
		},
		{
			name:                  "client-no-server-address",
			expectedServerAddress: "peerHostname",
			spans: []pb.Span{
				{
					Name: "span",
					Meta: map[string]string{
						"span.kind":     "client",
						"peer.hostname": "peerHostname",
					},
				},
			},
		},
		{
			name:                  "consumer",
			expectedServerAddress: "peerHostname",
			spans: []pb.Span{
				{
					Name: "span",
					Meta: map[string]string{
						"span.kind":     "consumer",
						"peer.hostname": "peerHostname",
					},
				},
			},
		},
		{
			name:                  "producer",
			expectedServerAddress: "peerHostname",
			spans: []pb.Span{
				{
					Name: "span",
					Meta: map[string]string{
						"span.kind":     "consumer",
						"peer.hostname": "peerHostname",
					},
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			payload := &pb.TracerPayload{
				Chunks: traceChunksFromSpans(tt.spans),
			}

			req := &http.Request{
				Header: http.Header{},
			}

			traces, _ := ToTraces(zap.NewNop(), payload, req, nil)
			for _, rs := range traces.ResourceSpans().All() {
				for _, ss := range rs.ScopeSpans().All() {
					for _, span := range ss.Spans().All() {
						val, _ := span.Attributes().Get("server.address")
						assert.Equal(t, tt.expectedServerAddress, val.Str())
					}
				}
			}
		})
	}
}
