// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

func TestOpenSearchTraceExporter(t *testing.T) {
	type requestHandler struct {
		ValidateReceivedDocuments func(*testing.T, int, []map[string]any)
		ResponseJSONPath          string
	}

	checkAndRespond := func(responsePath string) requestHandler {
		pass := func(t *testing.T, _ int, docs []map[string]any) {
			for _, doc := range docs {
				require.NotEmpty(t, doc)
			}
		}
		return requestHandler{pass, responsePath}
	}
	tests := []struct {
		Label                  string
		TracePath              string
		RequestHandlers        []requestHandler
		ValidateExporterReturn func(error)
	}{
		{
			"Round trip",
			"testdata/traces-sample-a.yaml",
			[]requestHandler{
				checkAndRespond("testdata/opensearch-response-no-error.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},
		{
			"Permanent error",
			"testdata/traces-sample-a.yaml",
			[]requestHandler{
				checkAndRespond("testdata/opensearch-response-permanent-error.json"),
			},
			func(err error) {
				require.True(t, consumererror.IsPermanent(err))
			},
		},
		{
			"Retryable error",
			"testdata/traces-sample-a.yaml",
			[]requestHandler{
				checkAndRespond("testdata/opensearch-response-retryable-error.json"),
				checkAndRespond("testdata/opensearch-response-retryable-succeeded.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},

		{
			"Retryable error, succeeds on second try",
			"testdata/traces-sample-a.yaml",
			[]requestHandler{
				checkAndRespond("testdata/opensearch-response-retryable-error.json"),
				checkAndRespond("testdata/opensearch-response-retryable-error-2-attempt.json"),
				checkAndRespond("testdata/opensearch-response-retryable-succeeded.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},
	}

	getReceivedDocuments := func(body io.ReadCloser) []map[string]any {
		var rtn []map[string]any
		var err error
		decoder := json.NewDecoder(body)
		for decoder.More() {
			var jsonData any
			err = decoder.Decode(&jsonData)
			require.NoError(t, err)
			require.NotNil(t, jsonData)

			strMap := jsonData.(map[string]any)
			if actionData, isBulkAction := strMap["create"]; isBulkAction {
				validateBulkAction(t, "ss4o_traces-default-namespace", actionData.(map[string]any))
			} else {
				rtn = append(rtn, strMap)
			}
		}
		return rtn
	}

	for _, tc := range tests {
		// Create HTTP listener
		requestCount := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			docs := getReceivedDocuments(r.Body)
			assert.LessOrEqualf(t, requestCount, len(tc.RequestHandlers), "Test case generated more requests than it has response for.")
			tc.RequestHandlers[requestCount].ValidateReceivedDocuments(t, requestCount, docs)

			w.WriteHeader(http.StatusOK)
			response, _ := os.ReadFile(tc.RequestHandlers[requestCount].ResponseJSONPath)
			_, err = w.Write(response)
			assert.NoError(t, err)

			requestCount++
		}))

		cfg := withDefaultConfig(func(config *Config) {
			config.Endpoint = ts.URL
			config.TimeoutSettings.Timeout = 0
		})

		// Create exporter
		f := NewFactory()
		exporter, err := f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
		require.NoError(t, err)

		// Initialize the exporter
		err = exporter.Start(t.Context(), componenttest.NewNopHost())
		require.NoError(t, err)

		// Load sample data
		traces, err := golden.ReadTraces(tc.TracePath)
		require.NoError(t, err)

		// Send it
		err = exporter.ConsumeTraces(t.Context(), traces)
		tc.ValidateExporterReturn(err)
		err = exporter.Shutdown(t.Context())
		require.NoError(t, err)
		ts.Close()
	}
}

func TestOpenSearchLogExporter(t *testing.T) {
	type requestHandler struct {
		ValidateReceivedDocuments func(*testing.T, int, []map[string]any)
		ResponseJSONPath          string
	}

	checkAndRespond := func(responsePath string) requestHandler {
		pass := func(t *testing.T, _ int, docs []map[string]any) {
			for _, doc := range docs {
				require.NotEmpty(t, doc)
			}
		}
		return requestHandler{pass, responsePath}
	}
	tests := []struct {
		Label                  string
		LogPath                string
		RequestHandlers        []requestHandler
		ValidateExporterReturn func(error)
	}{
		{
			"Round trip",
			"testdata/logs-sample-a.yaml",
			[]requestHandler{
				checkAndRespond("testdata/opensearch-response-no-error.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},
		{
			"Permanent error",
			"testdata/logs-sample-a.yaml",
			[]requestHandler{
				checkAndRespond("testdata/opensearch-response-permanent-error.json"),
			},
			func(err error) {
				require.True(t, consumererror.IsPermanent(err))
			},
		},
		{
			"Retryable error",
			"testdata/logs-sample-a.yaml",
			[]requestHandler{
				checkAndRespond("testdata/opensearch-response-retryable-error.json"),
				checkAndRespond("testdata/opensearch-response-retryable-succeeded.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},

		{
			"Retryable error, succeeds on second try",
			"testdata/logs-sample-a.yaml",
			[]requestHandler{
				checkAndRespond("testdata/opensearch-response-retryable-error.json"),
				checkAndRespond("testdata/opensearch-response-retryable-error-2-attempt.json"),
				checkAndRespond("testdata/opensearch-response-retryable-succeeded.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},
	}

	getReceivedDocuments := func(body io.ReadCloser) []map[string]any {
		var rtn []map[string]any
		var err error
		decoder := json.NewDecoder(body)
		for decoder.More() {
			var jsonData any
			err = decoder.Decode(&jsonData)
			require.NoError(t, err)
			require.NotNil(t, jsonData)

			strMap := jsonData.(map[string]any)
			if actionData, isBulkAction := strMap["create"]; isBulkAction {
				validateBulkAction(t, "ss4o_logs-default-namespace", actionData.(map[string]any))
			} else {
				rtn = append(rtn, strMap)
			}
		}
		return rtn
	}

	for _, tc := range tests {
		// Create HTTP listener
		requestCount := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			docs := getReceivedDocuments(r.Body)
			assert.LessOrEqualf(t, requestCount, len(tc.RequestHandlers), "Test case generated more requests than it has response for.")
			tc.RequestHandlers[requestCount].ValidateReceivedDocuments(t, requestCount, docs)

			w.WriteHeader(http.StatusOK)
			response, _ := os.ReadFile(tc.RequestHandlers[requestCount].ResponseJSONPath)
			_, err = w.Write(response)
			assert.NoError(t, err)

			requestCount++
		}))

		cfg := withDefaultConfig(func(config *Config) {
			config.Endpoint = ts.URL
			config.TimeoutSettings.Timeout = 0
		})

		// Create exporter
		f := NewFactory()
		exporter, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
		require.NoError(t, err)

		// Initialize the exporter
		err = exporter.Start(t.Context(), componenttest.NewNopHost())
		require.NoError(t, err)

		// Load sample data
		logs, err := golden.ReadLogs(tc.LogPath)
		require.NoError(t, err)

		// Send it
		err = exporter.ConsumeLogs(t.Context(), logs)
		tc.ValidateExporterReturn(err)
		err = exporter.Shutdown(t.Context())
		require.NoError(t, err)
		ts.Close()
	}
}

// validateBulkAction ensures the JSON object is to the correct index.
func validateBulkAction(t *testing.T, expectedIndex string, strMap map[string]any) {
	val, exists := strMap["_index"]
	require.True(t, exists)
	require.Equal(t, expectedIndex, val)
}

func TestOpenSearchTraceExporterOTelV1(t *testing.T) {
	var receivedDocs []map[string]any
	var bulkIndex string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		for decoder.More() {
			var jsonData any
			if !assert.NoError(t, decoder.Decode(&jsonData)) {
				return
			}
			strMap := jsonData.(map[string]any)
			if actionData, isBulkAction := strMap["create"]; isBulkAction {
				bulkIndex = actionData.(map[string]any)["_index"].(string)
			} else {
				receivedDocs = append(receivedDocs, strMap)
			}
		}
		w.WriteHeader(http.StatusOK)
		response, _ := os.ReadFile("testdata/opensearch-response-no-error.json")
		_, _ = w.Write(response)
	}))
	defer ts.Close()

	cfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = ts.URL
		config.TimeoutSettings.Timeout = 0
		config.Mode = "otel-v1"
	})

	f := NewFactory()
	exporter, err := f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	err = exporter.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	traces, err := golden.ReadTraces("testdata/traces-sample-a.yaml")
	require.NoError(t, err)
	err = exporter.ConsumeTraces(t.Context(), traces)
	require.NoError(t, err)

	// Verify index name
	assert.Equal(t, "otel-v1-apm-span", bulkIndex)

	// Verify document structure
	require.NotEmpty(t, receivedDocs)
	doc := receivedDocs[len(receivedDocs)-1] // last doc has richer data
	assert.Contains(t, doc, "traceId")
	assert.Contains(t, doc, "spanId")
	assert.Contains(t, doc, "durationInNanos")
	assert.Contains(t, doc, "startTime")
	assert.Contains(t, doc, "endTime")
	assert.Contains(t, doc, "resource")
	assert.Contains(t, doc, "instrumentationScope")

	// Verify status.code is numeric
	if status, ok := doc["status"].(map[string]any); ok {
		assert.IsType(t, float64(0), status["code"]) // JSON numbers decode as float64
	}

	err = exporter.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestOpenSearchLogExporterOTelV1(t *testing.T) {
	var receivedDocs []map[string]any
	var bulkIndex string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		for decoder.More() {
			var jsonData any
			if !assert.NoError(t, decoder.Decode(&jsonData)) {
				return
			}
			strMap := jsonData.(map[string]any)
			if actionData, isBulkAction := strMap["create"]; isBulkAction {
				bulkIndex = actionData.(map[string]any)["_index"].(string)
			} else {
				receivedDocs = append(receivedDocs, strMap)
			}
		}
		w.WriteHeader(http.StatusOK)
		response, _ := os.ReadFile("testdata/opensearch-response-no-error.json")
		_, _ = w.Write(response)
	}))
	defer ts.Close()

	cfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = ts.URL
		config.TimeoutSettings.Timeout = 0
		config.Mode = "otel-v1"
	})

	f := NewFactory()
	exporter, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	err = exporter.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	logs, err := golden.ReadLogs("testdata/logs-sample-a.yaml")
	require.NoError(t, err)
	err = exporter.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Verify index name
	assert.Equal(t, "otel-v1-logs", bulkIndex)

	// Verify document structure
	require.NotEmpty(t, receivedDocs)
	doc := receivedDocs[0]
	assert.Contains(t, doc, "@timestamp")
	assert.Contains(t, doc, "time")
	assert.Contains(t, doc, "observedTime")
	assert.Contains(t, doc, "severity")
	assert.Contains(t, doc, "body")
	assert.Contains(t, doc, "resource")
	assert.Contains(t, doc, "instrumentationScope")

	// Verify severity.number is numeric
	if sev, ok := doc["severity"].(map[string]any); ok {
		assert.IsType(t, float64(0), sev["number"])
	}

	// Verify flags is numeric
	assert.IsType(t, float64(0), doc["flags"])

	err = exporter.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestOpenSearchOTelV1_CustomIndex(t *testing.T) {
	var bulkIndex string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		for decoder.More() {
			var jsonData any
			_ = decoder.Decode(&jsonData)
			strMap := jsonData.(map[string]any)
			if actionData, isBulkAction := strMap["create"]; isBulkAction {
				bulkIndex = actionData.(map[string]any)["_index"].(string)
			}
		}
		w.WriteHeader(http.StatusOK)
		response, _ := os.ReadFile("testdata/opensearch-response-no-error.json")
		_, _ = w.Write(response)
	}))
	defer ts.Close()

	cfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = ts.URL
		config.TimeoutSettings.Timeout = 0
		config.Mode = "otel-v1"
		config.TracesIndex = "my-custom-traces"
	})

	f := NewFactory()
	exporter, err := f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(t.Context(), componenttest.NewNopHost()))

	traces, _ := golden.ReadTraces("testdata/traces-sample-a.yaml")
	require.NoError(t, exporter.ConsumeTraces(t.Context(), traces))

	assert.Equal(t, "my-custom-traces", bulkIndex)
	require.NoError(t, exporter.Shutdown(t.Context()))
}
