// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statusreporterextension

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestStatusHandler tests the HTTP handler for the status endpoint
func TestStatusHandler(t *testing.T) {
	// Create a statusReporterExtension instance with test configuration
	ext := &statusReporterExtension{
		config: &Config{
			MetricsEndpoint: "http://localhost:8888/metrics", // This won't be used as we'll mock the response
			StaleThreshold:  300,
			EngineID:        "test-engine",
			PodName:         "test-pod",
			Port:            8080,
			AuthEnabled:     false,
		},
		prevMetricsSent: make(map[string]int64),
		lastSuccessTime: make(map[string]int64),
		status: map[string]interface{}{
			"engine_id": "test-engine",
			"pod_name":  "test-pod",
			"traces": map[string]interface{}{
				"connection_status": "Active",
				"last_sync_ts":      1234567890,
				"error_message":     nil,
				"timestamp":         1234567890,
			},
			"metrics": map[string]interface{}{
				"connection_status": "Active",
				"last_sync_ts":      1234567890,
				"error_message":     nil,
				"timestamp":         1234567890,
			},
		},
	}

	// Create a request to pass to our handler
	req := httptest.NewRequest("GET", "/api/otel-status", nil)
	
	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	
	// Call the handler directly
	handler := http.HandlerFunc(ext.statusHandler)
	handler.ServeHTTP(rr, req)
	
	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check the content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want %v", contentType, "application/json")
	}
	
	// Check the response body
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response body: %v", err)
	}
	
	// Verify the response contains expected fields
	if engineID, ok := response["engine_id"].(string); !ok || engineID != "test-engine" {
		t.Errorf("Response missing or incorrect engine_id: %v", response["engine_id"])
	}
	
	if podName, ok := response["pod_name"].(string); !ok || podName != "test-pod" {
		t.Errorf("Response missing or incorrect pod_name: %v", response["pod_name"])
	}
	
	// Check traces section
	if traces, ok := response["traces"].(map[string]interface{}); !ok {
		t.Errorf("Response missing traces section")
	} else {
		if status, ok := traces["connection_status"].(string); !ok || status != "Active" {
			t.Errorf("Traces missing or incorrect connection_status: %v", traces["connection_status"])
		}
	}
	
	// Check metrics section
	if metrics, ok := response["metrics"].(map[string]interface{}); !ok {
		t.Errorf("Response missing metrics section")
	} else {
		if status, ok := metrics["connection_status"].(string); !ok || status != "Active" {
			t.Errorf("Metrics missing or incorrect connection_status: %v", metrics["connection_status"])
		}
	}
}

// TestStatusHandlerWithAuth tests the authentication mechanism
func TestStatusHandlerWithAuth(t *testing.T) {
	// Set environment variable for secret
	const secretValue = "test-secret"
	os.Setenv("LH_INSTANCE_SECRET", secretValue)
	defer os.Unsetenv("LH_INSTANCE_SECRET")
	
	// Create a statusReporterExtension instance with auth enabled
	ext := &statusReporterExtension{
		config: &Config{
			MetricsEndpoint: "http://localhost:8888/metrics",
			StaleThreshold:  300,
			EngineID:        "test-engine",
			PodName:         "test-pod",
			Port:            8080,
			AuthEnabled:     true,
			SecretValue:     "fallback-secret", // Should be overridden by env var
		},
		prevMetricsSent: make(map[string]int64),
		lastSuccessTime: make(map[string]int64),
		status: map[string]interface{}{
			"engine_id": "test-engine",
			"pod_name":  "test-pod",
			"traces": map[string]interface{}{
				"connection_status": "Active",
				"last_sync_ts":      1234567890,
				"error_message":     nil,
				"timestamp":         1234567890,
			},
			"metrics": map[string]interface{}{
				"connection_status": "Active",
				"last_sync_ts":      1234567890,
				"error_message":     nil,
				"timestamp":         1234567890,
			},
		},
	}
	
	// Test 1: Request without auth header (should fail)
	req := httptest.NewRequest("GET", "/api/otel-status", nil)
	rr := httptest.NewRecorder()
	
	handler := http.HandlerFunc(ext.statusHandler)
	handler.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler should return 401 for missing auth: got %v", status)
	}
	
	// Test 2: Request with incorrect auth header (should fail)
	req = httptest.NewRequest("GET", "/api/otel-status", nil)
	req.Header.Set("Secret", "wrong-secret")
	rr = httptest.NewRecorder()
	
	handler.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler should return 401 for wrong auth: got %v", status)
	}
	
	// Test 3: Request with correct auth header (should succeed)
	req = httptest.NewRequest("GET", "/api/otel-status", nil)
	req.Header.Set("Secret", secretValue)
	rr = httptest.NewRecorder()
	
	handler.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

// TestInvalidMethod tests that non-GET methods are rejected
func TestInvalidMethod(t *testing.T) {
	ext := &statusReporterExtension{
		config: &Config{
			MetricsEndpoint: "http://localhost:8888/metrics",
			StaleThreshold:  300,
			EngineID:        "test-engine",
			PodName:         "test-pod",
			Port:            8080,
			AuthEnabled:     false,
		},
		prevMetricsSent: make(map[string]int64),
		lastSuccessTime: make(map[string]int64),
	}
	
	// Test with POST method
	req := httptest.NewRequest("POST", "/api/otel-status", nil)
	rr := httptest.NewRecorder()
	
	handler := http.HandlerFunc(ext.statusHandler)
	handler.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler should return 405 for POST method: got %v", status)
	}
}

// TestCollectStatus tests the status collection logic
func TestCollectStatus(t *testing.T) {
	// Create a test server that returns mock metrics data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
# HELP otelcol_exporter_sent_spans_total Number of spans sent to destination.
# TYPE otelcol_exporter_sent_spans_total counter
otelcol_exporter_sent_spans_total{exporter="otlp",service_instance_id="test-instance"} 100
# HELP otelcol_exporter_send_failed_spans_total Number of spans failed to send to destination.
# TYPE otelcol_exporter_send_failed_spans_total counter
otelcol_exporter_send_failed_spans_total{exporter="otlp",service_instance_id="test-instance"} 0
# HELP otelcol_exporter_sent_metric_points_total Number of metric points sent to destination.
# TYPE otelcol_exporter_sent_metric_points_total counter
otelcol_exporter_sent_metric_points_total{exporter="otlp",service_instance_id="test-instance"} 200
# HELP otelcol_exporter_send_failed_metric_points_total Number of metric points failed to send to destination.
# TYPE otelcol_exporter_send_failed_metric_points_total counter
otelcol_exporter_send_failed_metric_points_total{exporter="otlp",service_instance_id="test-instance"} 0
# HELP rpc_client_duration_milliseconds Duration of outgoing RPC requests in milliseconds
# TYPE rpc_client_duration_milliseconds histogram
rpc_client_duration_milliseconds_count{rpc_service="opentelemetry.proto.collector.metrics.v1.MetricsService",rpc_method="Export",rpc_grpc_status_code="0"} 50
`))
	}))
	defer server.Close()
	
	ext := &statusReporterExtension{
		config: &Config{
			MetricsEndpoint: server.URL,
			StaleThreshold:  300,
			EngineID:        "test-engine",
			PodName:         "test-pod",
			Port:            8080,
			AuthEnabled:     false,
		},
		prevMetricsSent: make(map[string]int64),
		lastSuccessTime: make(map[string]int64),
	}
	
	// Collect status
	status := ext.collectStatus()
	
	// Verify the status
	if engineID, ok := status["engine_id"].(string); !ok || engineID != "test-engine" {
		t.Errorf("Status missing or incorrect engine_id: %v", status["engine_id"])
	}
	
	// Check traces status
	if traces, ok := status["traces"].(map[string]interface{}); !ok {
		t.Errorf("Status missing traces section")
	} else {
		if connStatus, ok := traces["connection_status"].(string); !ok || connStatus != "Active" {
			t.Errorf("Traces has incorrect connection_status: %v", traces["connection_status"])
		}
	}
	
	// Check metrics status
	if metrics, ok := status["metrics"].(map[string]interface{}); !ok {
		t.Errorf("Status missing metrics section")
	} else {
		if connStatus, ok := metrics["connection_status"].(string); !ok || connStatus != "Active" {
			t.Errorf("Metrics has incorrect connection_status: %v", metrics["connection_status"])
		}
	}
	
	// Verify that the metrics were parsed correctly
	if ext.prevMetricsSent["traces"] != 100 {
		t.Errorf("Failed to parse traces sent metric: got %v want %v", ext.prevMetricsSent["traces"], 100)
	}
	
	if ext.prevMetricsSent["metrics"] != 200 {
		t.Errorf("Failed to parse metrics sent metric: got %v want %v", ext.prevMetricsSent["metrics"], 200)
	}
}

// TestCollectStatusFailure tests the behavior when metrics endpoint is unreachable
func TestCollectStatusFailure(t *testing.T) {
	ext := &statusReporterExtension{
		config: &Config{
			MetricsEndpoint: "http://nonexistent-endpoint:12345/metrics",
			StaleThreshold:  300,
			EngineID:        "test-engine",
			PodName:         "test-pod",
			Port:            8080,
			AuthEnabled:     false,
		},
		prevMetricsSent: make(map[string]int64),
		lastSuccessTime: make(map[string]int64),
	}
	
	// Collect status (should handle the error gracefully)
	status := ext.collectStatus()
	
	// Verify the status contains fallback values
	if engineID, ok := status["engine_id"].(string); !ok || engineID != "test-engine" {
		t.Errorf("Status missing or incorrect engine_id: %v", status["engine_id"])
	}
	
	// Check traces status
	if traces, ok := status["traces"].(map[string]interface{}); !ok {
		t.Errorf("Status missing traces section")
	} else {
		if connStatus, ok := traces["connection_status"].(string); !ok || connStatus != "Unknown" {
			t.Errorf("Traces should have Unknown connection_status: %v", traces["connection_status"])
		}
	}
	
	// Check metrics status
	if metrics, ok := status["metrics"].(map[string]interface{}); !ok {
		t.Errorf("Status missing metrics section")
	} else {
		if connStatus, ok := metrics["connection_status"].(string); !ok || connStatus != "Unknown" {
			t.Errorf("Metrics should have Unknown connection_status: %v", metrics["connection_status"])
		}
	}
}

