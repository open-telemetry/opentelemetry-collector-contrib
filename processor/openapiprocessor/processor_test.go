// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestProcessTraces(t *testing.T) {
	tests := []struct {
		name                string
		config              *Config
		inputURL            string
		expectedTemplate    string
		expectedPeerService string
	}{
		{
			name: "match simple path",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
			},
			inputURL:            "/users",
			expectedTemplate:    "/users",
			expectedPeerService: "Pet Store API",
		},
		{
			name: "match path with parameter",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
			},
			inputURL:            "/users/123",
			expectedTemplate:    "/users/{userId}",
			expectedPeerService: "Pet Store API",
		},
		{
			name: "match path with multiple parameters",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
			},
			inputURL:            "/users/123/orders/456",
			expectedTemplate:    "/users/{userId}/orders/{orderId}",
			expectedPeerService: "Pet Store API",
		},
		{
			name: "no match",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
			},
			inputURL:            "/unknown/path",
			expectedTemplate:    "",
			expectedPeerService: "",
		},
		{
			name: "strip query params",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				IncludeQueryParams:   false,
			},
			inputURL:            "/users/123?include=orders",
			expectedTemplate:    "/users/{userId}",
			expectedPeerService: "Pet Store API",
		},
		{
			name: "custom peer service",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				PeerService:          "custom-service",
			},
			inputURL:            "/users",
			expectedTemplate:    "/users",
			expectedPeerService: "custom-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := newOpenAPIProcessor(tt.config, zap.NewNop())
			require.NoError(t, err)

			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetName("test-span")
			span.Attributes().PutStr("http.url", tt.inputURL)

			result, err := processor.processTraces(context.Background(), td)
			require.NoError(t, err)

			// Get the processed span
			processedSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			attrs := processedSpan.Attributes()

			if tt.expectedTemplate != "" {
				val, exists := attrs.Get(tt.config.URLTemplateAttribute)
				assert.True(t, exists, "url.template should exist")
				assert.Equal(t, tt.expectedTemplate, val.Str())
			} else {
				_, exists := attrs.Get(tt.config.URLTemplateAttribute)
				assert.False(t, exists, "url.template should not exist")
			}

			if tt.expectedPeerService != "" {
				val, exists := attrs.Get(tt.config.PeerServiceAttribute)
				assert.True(t, exists, "peer.service should exist")
				assert.Equal(t, tt.expectedPeerService, val.Str())
			} else {
				_, exists := attrs.Get(tt.config.PeerServiceAttribute)
				assert.False(t, exists, "peer.service should not exist")
			}
		})
	}
}

func TestProcessTracesOverwriteExisting(t *testing.T) {
	config := &Config{
		OpenAPIFile:          "testdata/openapi.yaml",
		URLAttribute:         "http.url",
		URLTemplateAttribute: "url.template",
		PeerServiceAttribute: "peer.service",
		OverwriteExisting:    false,
	}

	processor, err := newOpenAPIProcessor(config, zap.NewNop())
	require.NoError(t, err)

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr("http.url", "/users/123")
	span.Attributes().PutStr("url.template", "existing-template")

	result, err := processor.processTraces(context.Background(), td)
	require.NoError(t, err)

	processedSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	val, exists := processedSpan.Attributes().Get("url.template")
	assert.True(t, exists)
	assert.Equal(t, "existing-template", val.Str(), "should not overwrite existing value")
}

func TestProcessTracesOverwriteExistingEnabled(t *testing.T) {
	config := &Config{
		OpenAPIFile:          "testdata/openapi.yaml",
		URLAttribute:         "http.url",
		URLTemplateAttribute: "url.template",
		PeerServiceAttribute: "peer.service",
		OverwriteExisting:    true,
	}

	processor, err := newOpenAPIProcessor(config, zap.NewNop())
	require.NoError(t, err)

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr("http.url", "/users/123")
	span.Attributes().PutStr("url.template", "existing-template")

	result, err := processor.processTraces(context.Background(), td)
	require.NoError(t, err)

	processedSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	val, exists := processedSpan.Attributes().Get("url.template")
	assert.True(t, exists)
	assert.Equal(t, "/users/{userId}", val.Str(), "should overwrite existing value")
}

func TestFallbackURLAttributes(t *testing.T) {
	config := &Config{
		OpenAPIFile:          "testdata/openapi.yaml",
		URLAttribute:         "http.url",
		URLTemplateAttribute: "url.template",
		PeerServiceAttribute: "peer.service",
	}

	processor, err := newOpenAPIProcessor(config, zap.NewNop())
	require.NoError(t, err)

	// Test with url.path attribute instead of http.url
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr("url.path", "/users/123")

	result, err := processor.processTraces(context.Background(), td)
	require.NoError(t, err)

	processedSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	val, exists := processedSpan.Attributes().Get("url.template")
	assert.True(t, exists)
	assert.Equal(t, "/users/{userId}", val.Str())
}

func createTestTraces(urlAttr, urlValue string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr(urlAttr, urlValue)
	return td
}

func TestServerURLMatching(t *testing.T) {
	tests := []struct {
		name                string
		config              *Config
		inputURL            string
		expectedTemplate    string
		expectedPeerService string
	}{
		{
			name: "match full URL with server prefix",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				UseServerURLMatching: true,
			},
			inputURL:            "https://api.example.com/v1/users/123",
			expectedTemplate:    "/users/{userId}",
			expectedPeerService: "Pet Store API",
		},
		{
			name: "match staging server URL",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				UseServerURLMatching: true,
			},
			inputURL:            "https://staging.example.com/v1/users",
			expectedTemplate:    "/users",
			expectedPeerService: "Pet Store API",
		},
		{
			name: "match versioned server URL with v2",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				UseServerURLMatching: true,
			},
			inputURL:            "https://api.example.com/v2/users/123/orders/456",
			expectedTemplate:    "/users/{userId}/orders/{orderId}",
			expectedPeerService: "Pet Store API",
		},
		{
			name: "no match with server url matching",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				UseServerURLMatching: true,
			},
			inputURL:            "https://other-api.com/users/123",
			expectedTemplate:    "",
			expectedPeerService: "",
		},
		{
			name: "match full URL with query params stripped",
			config: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				UseServerURLMatching: true,
				IncludeQueryParams:   false,
			},
			inputURL:            "https://api.example.com/v1/users/123?include=orders",
			expectedTemplate:    "/users/{userId}",
			expectedPeerService: "Pet Store API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := newOpenAPIProcessor(tt.config, zap.NewNop())
			require.NoError(t, err)

			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetName("test-span")
			span.Attributes().PutStr("http.url", tt.inputURL)

			result, err := processor.processTraces(context.Background(), td)
			require.NoError(t, err)

			processedSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			attrs := processedSpan.Attributes()

			if tt.expectedTemplate != "" {
				val, exists := attrs.Get(tt.config.URLTemplateAttribute)
				assert.True(t, exists, "url.template should exist")
				assert.Equal(t, tt.expectedTemplate, val.Str())
			} else {
				_, exists := attrs.Get(tt.config.URLTemplateAttribute)
				assert.False(t, exists, "url.template should not exist")
			}

			if tt.expectedPeerService != "" {
				val, exists := attrs.Get(tt.config.PeerServiceAttribute)
				assert.True(t, exists, "peer.service should exist")
				assert.Equal(t, tt.expectedPeerService, val.Str())
			} else {
				_, exists := attrs.Get(tt.config.PeerServiceAttribute)
				assert.False(t, exists, "peer.service should not exist")
			}
		})
	}
}

func TestHTTPURLFetching(t *testing.T) {
	// Read the test OpenAPI spec
	specContent, err := os.ReadFile("testdata/openapi.yaml")
	require.NoError(t, err)

	// Create a test server that serves the OpenAPI spec
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(specContent)
	}))
	defer server.Close()

	// Create processor with HTTP URL
	cfg := &Config{
		OpenAPIFile:          server.URL + "/openapi.yaml",
		URLAttribute:         "http.url",
		URLTemplateAttribute: "url.template",
		PeerServiceAttribute: "peer.service",
	}

	processor, err := newOpenAPIProcessor(cfg, zap.NewNop())
	require.NoError(t, err)
	defer func() {
		_ = processor.shutdown(context.Background())
	}()

	// Test that the spec was loaded correctly
	spec := processor.spec.Load()
	assert.Equal(t, "Pet Store API", spec.GetTitle())
	assert.Len(t, spec.GetPaths(), 5)

	// Test processing a trace
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr("http.url", "/users/123")

	result, err := processor.processTraces(context.Background(), td)
	require.NoError(t, err)

	attrs := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	val, exists := attrs.Get("url.template")
	assert.True(t, exists)
	assert.Equal(t, "/users/{userId}", val.Str())
}

func TestHTTPURLRefresh(t *testing.T) {
	// Initial spec content
	initialSpec := `
openapi: "3.0.3"
info:
  title: Initial API
  version: "1.0"
paths:
  /users:
    get:
      summary: List users
      operationId: listUsers
      responses:
        "200":
          description: Successful response
`

	// Updated spec content with different paths
	updatedSpec := `
openapi: "3.0.3"
info:
  title: Updated API
  version: "2.0"
paths:
  /users:
    get:
      summary: List users
      operationId: listUsers
      responses:
        "200":
          description: Successful response
  /orders:
    get:
      summary: List orders
      operationId: listOrders
      responses:
        "200":
          description: Successful response
`

	specContent := initialSpec
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(specContent))
	}))
	defer server.Close()

	// Create processor with refresh interval
	cfg := &Config{
		OpenAPIFile:          server.URL + "/openapi.yaml",
		RefreshInterval:      100 * time.Millisecond,
		URLAttribute:         "http.url",
		URLTemplateAttribute: "url.template",
		PeerServiceAttribute: "peer.service",
	}

	processor, err := newOpenAPIProcessor(cfg, zap.NewNop())
	require.NoError(t, err)
	defer func() {
		_ = processor.shutdown(context.Background())
	}()

	// Verify initial spec
	spec := processor.spec.Load()
	assert.Equal(t, "Initial API", spec.GetTitle())
	assert.Len(t, spec.GetPaths(), 1)

	// Update the spec content
	specContent = updatedSpec

	// Wait for refresh to occur
	time.Sleep(200 * time.Millisecond)

	// Verify updated spec
	spec = processor.spec.Load()
	assert.Equal(t, "Updated API", spec.GetTitle())
	assert.Len(t, spec.GetPaths(), 2)
}
