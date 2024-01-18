// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func createTestServerTrace(name string, scheme string, host string, target string, method string) ptrace.Traces {
	trace := ptrace.NewTraces()

	trace.ResourceSpans().AppendEmpty()
	trace.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
	trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()

	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	span.SetName(name)
	span.SetKind(ptrace.SpanKindServer)
	span.Attributes().PutStr("url.scheme", scheme)

	if host != "" {
		span.Attributes().PutStr("server.address", host)
	}

	span.Attributes().PutStr("url.path", target)
	span.Attributes().PutStr("http.request.method", method)

	return trace
}

func createTestClientTrace(name string, urlFull string, method string) ptrace.Traces {
	trace := ptrace.NewTraces()

	trace.ResourceSpans().AppendEmpty()
	trace.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
	trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()

	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	span.SetName(name)
	span.SetKind(ptrace.SpanKindClient)
	span.Attributes().PutStr("url.full", urlFull)

	// Parse the URL to get the host
	parsedURL, _ := url.Parse(urlFull)

	span.Attributes().PutStr("server.address", parsedURL.Host)
	span.Attributes().PutStr("http.request.method", method)

	return trace
}

func verifySpanAttributes(t *testing.T, span ptrace.Span, expected map[string]string) {
	for k, v := range expected {
		attr, ok := span.Attributes().Get(k)
		require.True(t, ok)
		require.Equal(t, v, attr.AsString())
	}
}

func createTestServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/yaml")

		endsWithYaml := strings.HasSuffix(r.URL.Path, ".yaml")
		// If ends with yaml load from testdata
		if endsWithYaml {
			body, _ := os.ReadFile("testdata" + r.URL.Path)
			_, err := w.Write(body)
			require.NoError(t, err)
		} else {
			apiDirectoryResponse := &apiDirectoryResponse{
				Items: []string{
					"http://" + r.Host + "/petstore.yaml",
					"http://" + r.Host + "/gateway-service-a.yaml",
					"http://" + r.Host + "/gateway-service-b.yaml",
				},
			}
			jsonData, _ := json.Marshal(apiDirectoryResponse)
			_, err := w.Write(jsonData)
			require.NoError(t, err)
		}
	}))

	return server
}

func TestProcessTraces(t *testing.T) {

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIFilePaths:  []string{"testdata/petstore.yaml"},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestServerTrace("test", "http", "api.petstore.io", "/v2/pets/1", "GET")
	_ = oap.processTraces(context.Background(), trace)

	// Get the span from the trace
	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/pets/{petId}",
		"openapi.operation_id": "showPetById",
		"openapi.deprecated":   "false",
	})
}

func TestProcessClientTraces(t *testing.T) {

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIFilePaths:  []string{"testdata/petstore.yaml"},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestClientTrace("test", "http://api.petstore.io/v2/pets/1", "GET")
	_ = oap.processTraces(context.Background(), trace)

	// Get the span from the trace
	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/pets/{petId}",
		"openapi.operation_id": "showPetById",
		"openapi.deprecated":   "false",
	})
}

func TestProcessTracesWithExtractExtension(t *testing.T) {

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIFilePaths:  []string{"testdata/petstore.yaml"},
		Extensions:        []string{"x-operation-group"},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestServerTrace("test", "http", "api.petstore.io", "/v2/pets/1", "GET")
	_ = oap.processTraces(context.Background(), trace)

	// Get the span from the trace
	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":                "/pets/{petId}",
		"openapi.operation_id":      "showPetById",
		"openapi.deprecated":        "false",
		"openapi.x-operation-group": "pets",
	})
}

func TestDetermineHostFromServiceName(t *testing.T) {

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIFilePaths:  []string{"testdata/petstore.yaml"},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestServerTrace("test", "http", "", "/v2/pets/1", "GET")
	// Set service name on resourceSpan
	trace.ResourceSpans().At(0).Resource().Attributes().PutStr("service.name", "petstore")

	_ = oap.processTraces(context.Background(), trace)

	// Get the span from the trace
	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/pets/{petId}",
		"openapi.operation_id": "showPetById",
		"openapi.deprecated":   "false",
	})
}

func TestProcessTracesWithMultipleServicesOnSameHost(t *testing.T) {

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIFilePaths:  []string{"testdata/gateway-service-a.yaml", "testdata/gateway-service-b.yaml"},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	traceA := createTestServerTrace("test-a", "http", "api.petstore.io", "/v2/service-a/1", "GET")
	_ = oap.processTraces(context.Background(), traceA)

	// Get the span from the trace
	span := traceA.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/{petId}",
		"openapi.operation_id": "serviceAById",
		"openapi.deprecated":   "false",
	})

	traceB := createTestServerTrace("test-b", "http", "api.petstore.io", "/v2/service-b/1", "GET")
	_ = oap.processTraces(context.Background(), traceB)

	// Get the span from the trace
	span = traceB.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/{petId}",
		"openapi.operation_id": "serviceBById",
		"openapi.deprecated":   "false",
	})
}

func TestProcessTracesWithAllowHttpAndHttpsOptionTrue(t *testing.T) {

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIFilePaths:  []string{"testdata/petstore.yaml"},
		AllowHTTPAndHTTPS: true,
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestServerTrace("test", "https", "staging.petstore.io", "/v2/pets/1", "PATCH")
	_ = oap.processTraces(context.Background(), trace)

	// Get the span from the trace
	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/pets/{petId}",
		"openapi.operation_id": "updatePet",
		"openapi.deprecated":   "false",
	})
}

func TestLoadInlineApiSpec(t *testing.T) {

	// Read the OpenAPI spec from the file
	body, err := os.ReadFile("testdata/petstore.yaml")
	require.NoError(t, err)

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPISpecs:      []string{string(body)},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestServerTrace("test", "http", "api.petstore.io", "/v2/pets/1", "GET")

	_ = oap.processTraces(context.Background(), trace)

	// Get the span from the trace
	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/pets/{petId}",
		"openapi.operation_id": "showPetById",
		"openapi.deprecated":   "false",
	})
}

func TestLoadRemoteApiSpec(t *testing.T) {

	// Create httptest server to serve specs
	server := createTestServer(t)
	defer server.Close()

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIEndpoints:  []string{server.URL + "/petstore.yaml"},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestServerTrace("test", "http", "api.petstore.io", "/v2/pets/1", "GET")

	_ = oap.processTraces(context.Background(), trace)

	// Get the span from the trace
	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/pets/{petId}",
		"openapi.operation_id": "showPetById",
		"openapi.deprecated":   "false",
	})
}

func TestLoadApiDirectory(t *testing.T) {

	// Create httptest server to serve specs
	server := createTestServer(t)
	defer server.Close()

	cfg := &Config{
		APILoadTimeout:     defaultTimeout,
		APIReloadInterval:  defaultReloadInterval,
		OpenAPIDirectories: []string{server.URL},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestServerTrace("test", "http", "api.petstore.io", "/v2/pets/1", "GET")

	_ = oap.processTraces(context.Background(), trace)

	// Get the span from the trace
	span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	verifySpanAttributes(t, span, map[string]string{
		"http.route":           "/pets/{petId}",
		"openapi.operation_id": "showPetById",
		"openapi.deprecated":   "false",
	})
}

func BenchmarkProcessServerTraces(b *testing.B) {

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIFilePaths:  []string{"testdata/petstore.yaml"},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestServerTrace("test", "http", "api.petstore.io", "/v2/pets/1", "GET")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oap.processTraces(context.Background(), trace)
	}
}

func BenchmarkProcessClientTraces(b *testing.B) {

	cfg := &Config{
		APILoadTimeout:    defaultTimeout,
		APIReloadInterval: defaultReloadInterval,
		OpenAPIFilePaths:  []string{"testdata/petstore.yaml"},
	}

	tp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), *cfg)
	oap := tp.(*openAPIProcessor)

	trace := createTestClientTrace("test", "http://api.petstore.io/v2/pets/1", "GET")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oap.processTraces(context.Background(), trace)
	}
}
