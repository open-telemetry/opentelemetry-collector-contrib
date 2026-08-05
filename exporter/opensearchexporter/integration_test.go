// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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

// TestOpenSearchLogExporter_TransportFailureRetryable drives a real bulk flush
// against a server that never responds before the client timeout, so the flush
// fails at the transport level (context deadline / net timeout). The exporter
// must surface that as a retryable error rather than permanent, otherwise
// retry_on_failure would drop the batch. Retry is disabled here so the single
// synchronous attempt returns immediately. See #49208.
func TestOpenSearchLogExporter_TransportFailureRetryable(t *testing.T) {
	blocked := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-blocked // hold the request open past the client timeout
	}))
	defer func() {
		close(blocked)
		ts.Close()
	}()

	cfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = ts.URL
		config.TimeoutSettings.Timeout = 50 * time.Millisecond
		config.BackOffConfig.Enabled = false
	})

	f := NewFactory()
	exporter, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, exporter.Shutdown(t.Context())) }()

	logs, err := golden.ReadLogs("testdata/logs-sample-a.yaml")
	require.NoError(t, err)

	err = exporter.ConsumeLogs(t.Context(), logs)
	require.Error(t, err)
	require.False(t, consumererror.IsPermanent(err),
		"a transport/flush failure (timeout) must be retryable, not permanent")
}

// TestOpenSearchLogExporter_RetryPreservesWholeBatch is the regression test for the
// partial-loss variant of #49208. A flush failure fires the per-item OnFailure for
// every buffered record, and exporterhelper's OnError resolves the first
// consumererror it finds and retries only that payload. If the retryable error
// carried a single record, the retry would be narrowed to that one and the rest of
// the batch dropped. The first bulk request here fails at the transport level and
// the second succeeds, so every record in the batch must arrive on the retry.
func TestOpenSearchLogExporter_RetryPreservesWholeBatch(t *testing.T) {
	logs, err := golden.ReadLogs("testdata/logs-sample-a.yaml")
	require.NoError(t, err)
	wantRecords := logs.LogRecordCount()
	require.Greater(t, wantRecords, 1, "fixture must hold several records for this to mean anything")

	var mu sync.Mutex
	var attempts int
	var indexed int
	done := make(chan struct{})
	var closeOnce sync.Once

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		attempt := attempts
		mu.Unlock()

		if attempt == 1 {
			// Stall past the client timeout so the flush fails with a deadline,
			// which is the transport failure #49208 is about.
			time.Sleep(500 * time.Millisecond)
			return
		}

		body, readErr := io.ReadAll(r.Body)
		require.NoError(t, readErr)
		// The bulk body is action/document line pairs, so each action line is one record.
		var items []string
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
			if strings.Contains(line, `"create"`) || strings.Contains(line, `"index"`) {
				items = append(items, line)
			}
		}

		mu.Lock()
		indexed += len(items)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		var sb strings.Builder
		sb.WriteString(`{"took":1,"errors":false,"items":[`)
		for i := range items {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`{"create":{"_index":"logs","status":201}}`)
		}
		sb.WriteString(`]}`)
		_, _ = w.Write([]byte(sb.String()))
		closeOnce.Do(func() { close(done) })
	}))
	defer ts.Close()

	cfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = ts.URL
		config.TimeoutSettings.Timeout = 100 * time.Millisecond
		config.BackOffConfig.Enabled = true
		config.BackOffConfig.InitialInterval = 10 * time.Millisecond
		config.BackOffConfig.MaxInterval = 20 * time.Millisecond
		config.BackOffConfig.MaxElapsedTime = 10 * time.Second
	})

	f := NewFactory()
	exporter, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, exporter.Shutdown(t.Context())) }()

	require.NoError(t, exporter.ConsumeLogs(t.Context(), logs))

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the retried bulk request")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, attempts, 2, "the first attempt must fail and be retried")
	assert.Equal(t, wantRecords, indexed,
		"every record in the batch must be resent on the retry, not just the first")
}

// TestOpenSearchTraceExporter_RetryPreservesWholeBatch is the trace counterpart of
// TestOpenSearchLogExporter_RetryPreservesWholeBatch: the trace indexer shares the
// same per-item failure path, so it must also resend the whole batch on a retry.
func TestOpenSearchTraceExporter_RetryPreservesWholeBatch(t *testing.T) {
	traces, err := golden.ReadTraces("testdata/traces-sample-a.yaml")
	require.NoError(t, err)
	wantSpans := traces.SpanCount()
	require.Greater(t, wantSpans, 1, "fixture must hold several spans for this to mean anything")

	var mu sync.Mutex
	var attempts, indexed int
	done := make(chan struct{})
	var closeOnce sync.Once

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		attempt := attempts
		mu.Unlock()

		if attempt == 1 {
			// Stall past the client timeout so the flush fails with a deadline,
			// which is the transport failure #49208 is about.
			time.Sleep(500 * time.Millisecond)
			return
		}

		body, readErr := io.ReadAll(r.Body)
		require.NoError(t, readErr)
		var items int
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
			if strings.Contains(line, `"create"`) || strings.Contains(line, `"index"`) {
				items++
			}
		}

		mu.Lock()
		indexed += items
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		var sb strings.Builder
		sb.WriteString(`{"took":1,"errors":false,"items":[`)
		for i := 0; i < items; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`{"create":{"_index":"traces","status":201}}`)
		}
		sb.WriteString(`]}`)
		_, _ = w.Write([]byte(sb.String()))
		closeOnce.Do(func() { close(done) })
	}))
	defer ts.Close()

	cfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = ts.URL
		config.TimeoutSettings.Timeout = 100 * time.Millisecond
		config.BackOffConfig.Enabled = true
		config.BackOffConfig.InitialInterval = 10 * time.Millisecond
		config.BackOffConfig.MaxInterval = 20 * time.Millisecond
		config.BackOffConfig.MaxElapsedTime = 10 * time.Second
	})

	f := NewFactory()
	exporter, err := f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, exporter.Shutdown(t.Context())) }()

	require.NoError(t, exporter.ConsumeTraces(t.Context(), traces))

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the retried bulk request")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, attempts, 2, "the first attempt must fail and be retried")
	assert.Equal(t, wantSpans, indexed,
		"every span in the batch must be resent on the retry, not just the first")
}
