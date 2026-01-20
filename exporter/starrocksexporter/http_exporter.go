// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starrocksexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	streamLoadPathFormat = "/api/%s/%s/_stream_load"
)

// streamLoadResponse represents the Stream Load API response from StarRocks.
type streamLoadResponse struct {
	TxnId                int64  `json:"TxnId"`
	Label                string `json:"Label"`
	Status               string `json:"Status"`
	Message              string `json:"Message"`
	NumberTotalRows      int64  `json:"NumberTotalRows"`
	NumberLoadedRows     int64  `json:"NumberLoadedRows"`
	NumberFilteredRows   int64  `json:"NumberFilteredRows"`
	NumberUnselectedRows int64  `json:"NumberUnselectedRows"`
	LoadBytes            int64  `json:"LoadBytes"`
	LoadTimeMs           int64  `json:"LoadTimeMs"`
}

// httpLogsExporter handles HTTP-based data export using Stream Load API for logs.
type httpLogsExporter struct {
	client   *http.Client
	config   *Config
	logger   *zap.Logger
	settings component.TelemetrySettings
}

// newHTTPLogsExporter creates a new HTTP-based logs exporter.
func newHTTPLogsExporter(logger *zap.Logger, settings component.TelemetrySettings, cfg *Config) *httpLogsExporter {
	return &httpLogsExporter{
		config:   cfg,
		logger:   logger,
		settings: settings,
	}
}

func (e *httpLogsExporter) start(ctx context.Context, host component.Host) error {
	client, err := e.config.HTTP.ToClient(ctx, host.GetExtensions(), e.settings)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	e.client = client
	return nil
}

func (e *httpLogsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

// buildStreamLoadURL constructs the Stream Load API URL for a given table.
func (e *httpLogsExporter) buildStreamLoadURL(table string) string {
	endpoint := strings.TrimSuffix(e.config.HTTP.Endpoint, "/")
	return fmt.Sprintf("%s"+streamLoadPathFormat,
		endpoint,
		e.config.Database,
		table)
}

// generateLabel generates a unique label for the Stream Load request.
func (e *httpLogsExporter) generateLabel(prefix string) string {
	return fmt.Sprintf("otel_%s_%d", prefix, time.Now().UnixNano())
}

// pushLogsData exports log data via Stream Load API.
func (e *httpLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	// Convert logs to JSON format
	jsonData, err := e.convertLogsToJSON(ld)
	if err != nil {
		return fmt.Errorf("failed to convert logs to JSON: %w", err)
	}

	if len(jsonData) == 0 {
		return nil
	}

	url := e.buildStreamLoadURL(e.config.LogsTableName)
	label := e.generateLabel("logs")

	// Build HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for Stream Load
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("format", "json")
	req.Header.Set("strip_outer_array", "true")
	req.Header.Set("label", label)
	req.Header.Set("timeout", fmt.Sprintf("%.0f", e.config.StreamLoadTimeout.Seconds()))
	if e.config.MaxFilterRatio > 0 {
		req.Header.Set("max_filter_ratio", fmt.Sprintf("%.4f", e.config.MaxFilterRatio))
	}

	// Set basic authentication
	req.SetBasicAuth(e.config.Username, string(e.config.Password))

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var slResp streamLoadResponse
	if err := json.Unmarshal(body, &slResp); err != nil {
		return fmt.Errorf("failed to decode response: %w, body: %s", err, string(body))
	}

	// Check status and handle accordingly
	return e.handleStreamLoadResponse(slResp, url)
}

// handleStreamLoadResponse processes the Stream Load API response.
func (e *httpLogsExporter) handleStreamLoadResponse(resp streamLoadResponse, url string) error {
	switch resp.Status {
	case "Success":
		e.logger.Debug("Stream Load success",
			zap.String("table", e.config.LogsTableName),
			zap.String("label", resp.Label),
			zap.Int64("rows", resp.NumberLoadedRows),
			zap.Int64("bytes", resp.LoadBytes),
			zap.Int64("time_ms", resp.LoadTimeMs))
		return nil

	case "Publish Timeout":
		// Data loaded but not yet visible - don't retry, considered success
		e.logger.Debug("Stream Load publish timeout (data loaded, not yet visible)",
			zap.String("table", e.config.LogsTableName),
			zap.String("label", resp.Label))
		return nil

	case "Label Already Exists":
		// Duplicate label - treat as success (idempotent)
		e.logger.Debug("Stream Load label already exists (idempotent)",
			zap.String("table", e.config.LogsTableName),
			zap.String("label", resp.Label))
		return nil

	case "Fail", "Error":
		// Check for retryable errors
		msg := strings.ToLower(resp.Message)
		if strings.Contains(msg, "timeout") ||
			strings.Contains(msg, "connection") ||
			strings.Contains(msg, "temporary") {
			return fmt.Errorf("retryable Stream Load error: %s (Label: %s)", resp.Message, resp.Label)
		}
		// Permanent error
		return consumererror.NewPermanent(fmt.Errorf("Stream Load failed: %s (Label: %s, URL: %s)", resp.Message, resp.Label, url))

	default:
		return fmt.Errorf("unknown Stream Load status: %s - %s (Label: %s)", resp.Status, resp.Message, resp.Label)
	}
}

// convertLogsToJSON converts plog.Logs to JSON array format for Stream Load.
func (e *httpLogsExporter) convertLogsToJSON(ld plog.Logs) ([]byte, error) {
	var logEntries []map[string]interface{}

	rsLogs := ld.ResourceLogs()
	for i := 0; i < rsLogs.Len(); i++ {
		rl := rsLogs.At(i)
		res := rl.Resource()
		resURL := rl.SchemaUrl()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrJSON, err := internal.AttributesToJSON(resAttr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resource attributes to JSON: %w", err)
		}

		slLogs := rl.ScopeLogs()
		for j := 0; j < slLogs.Len(); j++ {
			scopeLog := slLogs.At(j)
			scope := scopeLog.Scope()
			scopeAttrJSON, err := internal.AttributesToJSON(scope.Attributes())
			if err != nil {
				return nil, fmt.Errorf("failed to convert scope attributes to JSON: %w", err)
			}

			logRecords := scopeLog.LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				r := logRecords.At(k)
				logAttrJSON, err := internal.AttributesToJSON(r.Attributes())
				if err != nil {
					return nil, fmt.Errorf("failed to convert log attributes to JSON: %w", err)
				}

				timestamp := r.Timestamp()
				if timestamp == 0 {
					timestamp = r.ObservedTimestamp()
				}

				logEntry := map[string]interface{}{
					"ServiceName":        serviceName,
					"Timestamp":          timestamp.AsTime().Format("2006-01-02 15:04:05"),
					"TraceId":            traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					"SpanId":             traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					"TraceFlags":         uint8(r.Flags()),
					"SeverityText":       r.SeverityText(),
					"SeverityNumber":     uint8(r.SeverityNumber()),
					"Body":               r.Body().AsString(),
					"ResourceSchemaURL":  resURL,
					"ResourceAttributes": string(resAttrJSON),
					"ScopeName":          scope.Name(),
					"ScopeVersion":       scope.Version(),
					"ScopeAttributes":    string(scopeAttrJSON),
					"LogAttributes":      string(logAttrJSON),
				}

				logEntries = append(logEntries, logEntry)
			}
		}
	}

	if len(logEntries) == 0 {
		return nil, nil
	}

	return json.Marshal(logEntries)
}

// httpTracesExporter handles HTTP-based data export using Stream Load API for traces.
type httpTracesExporter struct {
	client   *http.Client
	config   *Config
	logger   *zap.Logger
	settings component.TelemetrySettings
}

// newHTTPTracesExporter creates a new HTTP-based traces exporter.
func newHTTPTracesExporter(logger *zap.Logger, settings component.TelemetrySettings, cfg *Config) *httpTracesExporter {
	return &httpTracesExporter{
		config:   cfg,
		logger:   logger,
		settings: settings,
	}
}

func (e *httpTracesExporter) start(ctx context.Context, host component.Host) error {
	client, err := e.config.HTTP.ToClient(ctx, host.GetExtensions(), e.settings)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	e.client = client
	return nil
}

func (e *httpTracesExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

// buildStreamLoadURL constructs the Stream Load API URL for a given table.
func (e *httpTracesExporter) buildStreamLoadURL(table string) string {
	endpoint := strings.TrimSuffix(e.config.HTTP.Endpoint, "/")
	return fmt.Sprintf("%s"+streamLoadPathFormat,
		endpoint,
		e.config.Database,
		table)
}

// pushTraceData exports trace data via Stream Load API.
func (e *httpTracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	// Convert traces to JSON format
	jsonData, err := e.convertTracesToJSON(td)
	if err != nil {
		return fmt.Errorf("failed to convert traces to JSON: %w", err)
	}

	if len(jsonData) == 0 {
		return nil
	}

	url := e.buildStreamLoadURL(e.config.TracesTableName)
	label := e.generateLabel("traces")

	// Build HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for Stream Load
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("format", "json")
	req.Header.Set("strip_outer_array", "true")
	req.Header.Set("label", label)
	req.Header.Set("timeout", fmt.Sprintf("%.0f", e.config.StreamLoadTimeout.Seconds()))
	if e.config.MaxFilterRatio > 0 {
		req.Header.Set("max_filter_ratio", fmt.Sprintf("%.4f", e.config.MaxFilterRatio))
	}

	// Set basic authentication
	req.SetBasicAuth(e.config.Username, string(e.config.Password))

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var slResp streamLoadResponse
	if err := json.Unmarshal(body, &slResp); err != nil {
		return fmt.Errorf("failed to decode response: %w, body: %s", err, string(body))
	}

	// Check status and handle accordingly
	return e.handleStreamLoadResponse(slResp, url)
}

// handleStreamLoadResponse processes the Stream Load API response.
func (e *httpTracesExporter) handleStreamLoadResponse(resp streamLoadResponse, url string) error {
	switch resp.Status {
	case "Success":
		e.logger.Debug("Stream Load success",
			zap.String("table", e.config.TracesTableName),
			zap.String("label", resp.Label),
			zap.Int64("rows", resp.NumberLoadedRows),
			zap.Int64("bytes", resp.LoadBytes),
			zap.Int64("time_ms", resp.LoadTimeMs))
		return nil

	case "Publish Timeout":
		e.logger.Debug("Stream Load publish timeout (data loaded, not yet visible)",
			zap.String("table", e.config.TracesTableName),
			zap.String("label", resp.Label))
		return nil

	case "Label Already Exists":
		e.logger.Debug("Stream Load label already exists (idempotent)",
			zap.String("table", e.config.TracesTableName),
			zap.String("label", resp.Label))
		return nil

	case "Fail", "Error":
		msg := strings.ToLower(resp.Message)
		if strings.Contains(msg, "timeout") ||
			strings.Contains(msg, "connection") ||
			strings.Contains(msg, "temporary") {
			return fmt.Errorf("retryable Stream Load error: %s (Label: %s)", resp.Message, resp.Label)
		}
		return consumererror.NewPermanent(fmt.Errorf("Stream Load failed: %s (Label: %s, URL: %s)", resp.Message, resp.Label, url))

	default:
		return fmt.Errorf("unknown Stream Load status: %s - %s (Label: %s)", resp.Status, resp.Message, resp.Label)
	}
}

// generateLabel generates a unique label for the Stream Load request.
func (e *httpTracesExporter) generateLabel(prefix string) string {
	return fmt.Sprintf("otel_%s_%d", prefix, time.Now().UnixNano())
}

// convertTracesToJSON converts ptrace.Traces to JSON array format for Stream Load.
func (e *httpTracesExporter) convertTracesToJSON(td ptrace.Traces) ([]byte, error) {
	var spanEntries []map[string]interface{}

	rsSpans := td.ResourceSpans()
	for i := 0; i < rsSpans.Len(); i++ {
		rs := rsSpans.At(i)
		res := rs.Resource()
		resURL := rs.SchemaUrl()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrJSON, err := internal.AttributesToJSON(resAttr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resource attributes to JSON: %w", err)
		}

		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)
			scope := ss.Scope()
			scopeAttrJSON, err := internal.AttributesToJSON(scope.Attributes())
			if err != nil {
				return nil, fmt.Errorf("failed to convert scope attributes to JSON: %w", err)
			}

			spanSlice := ss.Spans()
			for k := 0; k < spanSlice.Len(); k++ {
				span := spanSlice.At(k)
				spanAttrJSON, err := internal.AttributesToJSON(span.Attributes())
				if err != nil {
					return nil, fmt.Errorf("failed to convert span attributes to JSON: %w", err)
				}

				spanLinksJSON, err := e.convertSpanLinks(span.Links())
				if err != nil {
					return nil, fmt.Errorf("failed to convert span links: %w", err)
				}

				spanEventsJSON, err := e.convertSpanEvents(span.Events())
				if err != nil {
					return nil, fmt.Errorf("failed to convert span events: %w", err)
				}

				spanEntry := map[string]interface{}{
					"ServiceName":        serviceName,
					"TraceId":            traceutil.TraceIDToHexOrEmptyString(span.TraceID()),
					"SpanId":             traceutil.SpanIDToHexOrEmptyString(span.SpanID()),
					"ParentSpanId":       traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
					"TraceState":         span.TraceState().AsRaw(),
					"SpanName":           span.Name(),
					"SpanKind":           span.Kind().String(),
					"StartTime":          span.StartTimestamp().AsTime().Format("2006-01-02 15:04:05"),
					"EndTime":            span.EndTimestamp().AsTime().Format("2006-01-02 15:04:05"),
					"Duration":           (span.EndTimestamp() - span.StartTimestamp()).AsTime().String(),
					"Status":             span.Status().Code().String(),
					"StatusMessage":      span.Status().Message(),
					"ResourceSchemaURL":  resURL,
					"ResourceAttributes": string(resAttrJSON),
					"ScopeName":          scope.Name(),
					"ScopeVersion":       scope.Version(),
					"ScopeAttributes":    string(scopeAttrJSON),
					"SpanAttributes":     string(spanAttrJSON),
					"SpanLinks":          spanLinksJSON,
					"SpanEvents":         spanEventsJSON,
				}

				spanEntries = append(spanEntries, spanEntry)
			}
		}
	}

	if len(spanEntries) == 0 {
		return nil, nil
	}

	return json.Marshal(spanEntries)
}

// convertSpanLinks converts span links to JSON format.
func (e *httpTracesExporter) convertSpanLinks(sl ptrace.SpanLinkSlice) (string, error) {
	if sl.Len() == 0 {
		return "[]", nil
	}

	links := make([]map[string]interface{}, 0, sl.Len())
	for i := 0; i < sl.Len(); i++ {
		link := sl.At(i)
		linkJSON := map[string]interface{}{
			"TraceId":    traceutil.TraceIDToHexOrEmptyString(link.TraceID()),
			"SpanId":     traceutil.SpanIDToHexOrEmptyString(link.SpanID()),
			"TraceState": link.TraceState().AsRaw(),
			"Attributes": link.Attributes().AsRaw(),
		}
		links = append(links, linkJSON)
	}

	data, err := json.Marshal(links)
	if err != nil {
		return "[]", fmt.Errorf("failed to marshal span links: %w", err)
	}
	return string(data), nil
}

// convertSpanEvents converts span events to JSON format.
func (e *httpTracesExporter) convertSpanEvents(se ptrace.SpanEventSlice) (string, error) {
	if se.Len() == 0 {
		return "[]", nil
	}

	events := make([]map[string]interface{}, 0, se.Len())
	for i := 0; i < se.Len(); i++ {
		event := se.At(i)
		eventJSON := map[string]interface{}{
			"Name":       event.Name(),
			"Time":       event.Timestamp().AsTime().Format("2006-01-02 15:04:05"),
			"Attributes": event.Attributes().AsRaw(),
		}
		events = append(events, eventJSON)
	}

	data, err := json.Marshal(events)
	if err != nil {
		return "[]", fmt.Errorf("failed to marshal span events: %w", err)
	}
	return string(data), nil
}

// httpMetricsExporter handles HTTP-based data export using Stream Load API for metrics.
type httpMetricsExporter struct {
	client   *http.Client
	config   *Config
	logger   *zap.Logger
	settings component.TelemetrySettings
}

// newHTTPMetricsExporter creates a new HTTP-based metrics exporter.
func newHTTPMetricsExporter(logger *zap.Logger, settings component.TelemetrySettings, cfg *Config) *httpMetricsExporter {
	return &httpMetricsExporter{
		config:   cfg,
		logger:   logger,
		settings: settings,
	}
}

func (e *httpMetricsExporter) start(ctx context.Context, host component.Host) error {
	client, err := e.config.HTTP.ToClient(ctx, host.GetExtensions(), e.settings)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	e.client = client
	return nil
}

func (e *httpMetricsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

// buildStreamLoadURL constructs the Stream Load API URL for a given table.
func (e *httpMetricsExporter) buildStreamLoadURL(table string) string {
	endpoint := strings.TrimSuffix(e.config.HTTP.Endpoint, "/")
	return fmt.Sprintf("%s"+streamLoadPathFormat,
		endpoint,
		e.config.Database,
		table)
}

// pushMetricsData exports metric data via Stream Load API.
func (e *httpMetricsExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	// Convert metrics to JSON format grouped by metric type
	metricsByType, err := e.convertMetricsToJSONByType(md)
	if err != nil {
		return fmt.Errorf("failed to convert metrics to JSON: %w", err)
	}

	var errs []error

	// Export each metric type to its respective table
	for metricType, jsonData := range metricsByType {
		if len(jsonData) == 0 {
			continue
		}

		tableName := e.getTableNameForMetricType(metricType)
		if tableName == "" {
			continue
		}

		url := e.buildStreamLoadURL(tableName)
		label := e.generateLabel(fmt.Sprintf("metrics_%s", metricType))

		// Build HTTP request
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(jsonData))
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create request for %s: %w", metricType, err))
			continue
		}

		// Set headers for Stream Load
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Expect", "100-continue")
		req.Header.Set("format", "json")
		req.Header.Set("strip_outer_array", "true")
		req.Header.Set("label", label)
		req.Header.Set("timeout", fmt.Sprintf("%.0f", e.config.StreamLoadTimeout.Seconds()))
		if e.config.MaxFilterRatio > 0 {
			req.Header.Set("max_filter_ratio", fmt.Sprintf("%.4f", e.config.MaxFilterRatio))
		}

		// Set basic authentication
		req.SetBasicAuth(e.config.Username, string(e.config.Password))

		// Execute request
		resp, err := e.client.Do(req)
		if err != nil {
			errs = append(errs, fmt.Errorf("HTTP request failed for %s: %w", metricType, err))
			continue
		}
		defer func() {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to read response body for %s: %w", metricType, err))
			continue
		}

		// Parse response
		var slResp streamLoadResponse
		if err := json.Unmarshal(body, &slResp); err != nil {
			errs = append(errs, fmt.Errorf("failed to decode response for %s: %w, body: %s", metricType, err, string(body)))
			continue
		}

		// Handle response
		if err := e.handleStreamLoadResponse(slResp, url, metricType); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("metrics export encountered errors: %v", errs)
	}

	return nil
}

// getTableNameForMetricType returns the table name for a given metric type.
func (e *httpMetricsExporter) getTableNameForMetricType(metricType string) string {
	switch metricType {
	case "gauge":
		return e.config.MetricsTables.Gauge.Name
	case "sum":
		return e.config.MetricsTables.Sum.Name
	case "summary":
		return e.config.MetricsTables.Summary.Name
	case "histogram":
		return e.config.MetricsTables.Histogram.Name
	case "exponential_histogram":
		return e.config.MetricsTables.ExponentialHistogram.Name
	default:
		return ""
	}
}

// handleStreamLoadResponse processes the Stream Load API response.
func (e *httpMetricsExporter) handleStreamLoadResponse(resp streamLoadResponse, url, metricType string) error {
	switch resp.Status {
	case "Success":
		e.logger.Debug("Stream Load success",
			zap.String("metric_type", metricType),
			zap.String("label", resp.Label),
			zap.Int64("rows", resp.NumberLoadedRows),
			zap.Int64("bytes", resp.LoadBytes),
			zap.Int64("time_ms", resp.LoadTimeMs))
		return nil

	case "Publish Timeout":
		e.logger.Debug("Stream Load publish timeout (data loaded, not yet visible)",
			zap.String("metric_type", metricType),
			zap.String("label", resp.Label))
		return nil

	case "Label Already Exists":
		e.logger.Debug("Stream Load label already exists (idempotent)",
			zap.String("metric_type", metricType),
			zap.String("label", resp.Label))
		return nil

	case "Fail", "Error":
		msg := strings.ToLower(resp.Message)
		if strings.Contains(msg, "timeout") ||
			strings.Contains(msg, "connection") ||
			strings.Contains(msg, "temporary") {
			return fmt.Errorf("retryable Stream Load error for %s: %s (Label: %s)", metricType, resp.Message, resp.Label)
		}
		return consumererror.NewPermanent(fmt.Errorf("Stream Load failed for %s: %s (Label: %s, URL: %s)", metricType, resp.Message, resp.Label, url))

	default:
		return fmt.Errorf("unknown Stream Load status for %s: %s - %s (Label: %s)", metricType, resp.Status, resp.Message, resp.Label)
	}
}

// generateLabel generates a unique label for the Stream Load request.
func (e *httpMetricsExporter) generateLabel(prefix string) string {
	return fmt.Sprintf("otel_%s_%d", prefix, time.Now().UnixNano())
}

// convertMetricsToJSONByType converts pmetric.Metrics to JSON format grouped by metric type.
func (e *httpMetricsExporter) convertMetricsToJSONByType(md pmetric.Metrics) (map[string][]byte, error) {
	result := make(map[string][]byte)

	type metricsBatch struct {
		metrics []map[string]interface{}
	}

	batches := make(map[string]*metricsBatch)
	batches["gauge"] = &metricsBatch{}
	batches["sum"] = &metricsBatch{}
	batches["summary"] = &metricsBatch{}
	batches["histogram"] = &metricsBatch{}
	batches["exponential_histogram"] = &metricsBatch{}

	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		res := rm.At(i).Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrJSON, err := internal.AttributesToJSON(resAttr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resource attributes to JSON: %w", err)
		}

		scm := rm.At(i).ScopeMetrics()
		for j := 0; j < scm.Len(); j++ {
			scope := scm.At(j).Scope()
			scopeAttrJSON, err := internal.AttributesToJSON(scope.Attributes())
			if err != nil {
				return nil, fmt.Errorf("failed to convert scope attributes to JSON: %w", err)
			}

			metrics := scm.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				metricType := getMetricTypeName(metric)

				batch := batches[metricType]
				if batch == nil {
					continue
				}

				metricEntry := map[string]interface{}{
					"ServiceName":        serviceName,
					"MetricName":         metric.Name(),
					"MetricType":         metricType,
					"MetricDescription":  metric.Description(),
					"MetricUnit":         metric.Unit(),
					"ResourceAttributes": string(resAttrJSON),
					"ScopeName":          scope.Name(),
					"ScopeVersion":       scope.Version(),
					"ScopeAttributes":    string(scopeAttrJSON),
				}

				// Add metric-specific data - iterate through all data points
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					gauge := metric.Gauge()
					for dpIdx := 0; dpIdx < gauge.DataPoints().Len(); dpIdx++ {
						dataPoint := gauge.DataPoints().At(dpIdx)
						entry := copyMap(metricEntry)
						entry["Value"] = e.extractGaugeValue(dataPoint)
						entry["Attributes"] = dataPoint.Attributes().AsRaw()
						entry["StartTime"] = dataPoint.StartTimestamp().AsTime().Format("2006-01-02 15:04:05")
						entry["Timestamp"] = dataPoint.Timestamp().AsTime().Format("2006-01-02 15:04:05")
						batch.metrics = append(batch.metrics, entry)
					}
				case pmetric.MetricTypeSum:
					sum := metric.Sum()
					for dpIdx := 0; dpIdx < sum.DataPoints().Len(); dpIdx++ {
						dataPoint := sum.DataPoints().At(dpIdx)
						entry := copyMap(metricEntry)
						entry["Value"] = e.extractSumValue(dataPoint)
						entry["IsMonotonic"] = sum.IsMonotonic()
						entry["Attributes"] = dataPoint.Attributes().AsRaw()
						entry["StartTime"] = dataPoint.StartTimestamp().AsTime().Format("2006-01-02 15:04:05")
						entry["Timestamp"] = dataPoint.Timestamp().AsTime().Format("2006-01-02 15:04:05")
						batch.metrics = append(batch.metrics, entry)
					}
				case pmetric.MetricTypeSummary:
					summary := metric.Summary()
					for dpIdx := 0; dpIdx < summary.DataPoints().Len(); dpIdx++ {
						dataPoint := summary.DataPoints().At(dpIdx)
						entry := copyMap(metricEntry)
						entry["SummaryData"] = e.extractSummaryValue(dataPoint)
						entry["Attributes"] = dataPoint.Attributes().AsRaw()
						entry["StartTime"] = dataPoint.StartTimestamp().AsTime().Format("2006-01-02 15:04:05")
						entry["Timestamp"] = dataPoint.Timestamp().AsTime().Format("2006-01-02 15:04:05")
						batch.metrics = append(batch.metrics, entry)
					}
				case pmetric.MetricTypeHistogram:
					histogram := metric.Histogram()
					for dpIdx := 0; dpIdx < histogram.DataPoints().Len(); dpIdx++ {
						dataPoint := histogram.DataPoints().At(dpIdx)
						entry := copyMap(metricEntry)
						entry["HistogramData"] = e.extractHistogramValue(dataPoint)
						entry["Attributes"] = dataPoint.Attributes().AsRaw()
						entry["StartTime"] = dataPoint.StartTimestamp().AsTime().Format("2006-01-02 15:04:05")
						entry["Timestamp"] = dataPoint.Timestamp().AsTime().Format("2006-01-02 15:04:05")
						batch.metrics = append(batch.metrics, entry)
					}
				case pmetric.MetricTypeExponentialHistogram:
					expHist := metric.ExponentialHistogram()
					for dpIdx := 0; dpIdx < expHist.DataPoints().Len(); dpIdx++ {
						dataPoint := expHist.DataPoints().At(dpIdx)
						entry := copyMap(metricEntry)
						entry["ExponentialHistogramData"] = e.extractExponentialHistogramValue(dataPoint)
						entry["Attributes"] = dataPoint.Attributes().AsRaw()
						entry["StartTime"] = dataPoint.StartTimestamp().AsTime().Format("2006-01-02 15:04:05")
						entry["Timestamp"] = dataPoint.Timestamp().AsTime().Format("2006-01-02 15:04:05")
						batch.metrics = append(batch.metrics, entry)
					}
				default:
					// For unknown types, just add the base entry
					batch.metrics = append(batch.metrics, metricEntry)
				}
			}
		}
	}

	// Marshal each batch to JSON
	for metricType, batch := range batches {
		if len(batch.metrics) > 0 {
			data, err := json.Marshal(batch.metrics)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal %s metrics: %w", metricType, err)
			}
			result[metricType] = data
		}
	}

	return result, nil
}

// getMetricTypeName returns the metric type name for a given metric.
func getMetricTypeName(metric pmetric.Metric) string {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return "gauge"
	case pmetric.MetricTypeSum:
		return "sum"
	case pmetric.MetricTypeSummary:
		return "summary"
	case pmetric.MetricTypeHistogram:
		return "histogram"
	case pmetric.MetricTypeExponentialHistogram:
		return "exponential_histogram"
	default:
		return "unknown"
	}
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]interface{}) map[string]interface{} {
	copied := make(map[string]interface{}, len(m))
	for k, v := range m {
		copied[k] = v
	}
	return copied
}

// extractGaugeValue extracts gauge data point value.
func (e *httpMetricsExporter) extractGaugeValue(dp pmetric.NumberDataPoint) interface{} {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return dp.IntValue()
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	default:
		return nil
	}
}

// extractSumValue extracts sum data point value.
func (e *httpMetricsExporter) extractSumValue(dp pmetric.NumberDataPoint) interface{} {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return dp.IntValue()
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	default:
		return nil
	}
}

// extractSummaryValue extracts summary data point values.
func (e *httpMetricsExporter) extractSummaryValue(dp pmetric.SummaryDataPoint) map[string]interface{} {
	quantiles := make([]map[string]interface{}, 0, dp.QuantileValues().Len())
	for i := 0; i < dp.QuantileValues().Len(); i++ {
		qv := dp.QuantileValues().At(i)
		quantiles = append(quantiles, map[string]interface{}{
			"quantile": qv.Quantile(),
			"value":    qv.Value(),
		})
	}

	return map[string]interface{}{
		"count":     dp.Count(),
		"sum":       dp.Sum(),
		"quantiles": quantiles,
	}
}

// extractHistogramValue extracts histogram data point values.
func (e *httpMetricsExporter) extractHistogramValue(dp pmetric.HistogramDataPoint) map[string]interface{} {
	bounds := make([]float64, 0, dp.BucketCounts().Len())
	explicitBounds := dp.ExplicitBounds()
	for i := 0; i < explicitBounds.Len(); i++ {
		bounds = append(bounds, explicitBounds.At(i))
	}

	counts := make([]uint64, 0, dp.BucketCounts().Len())
	for i := 0; i < dp.BucketCounts().Len(); i++ {
		counts = append(counts, dp.BucketCounts().At(i))
	}

	return map[string]interface{}{
		"count":           dp.Count(),
		"sum":             dp.Sum(),
		"min":             dp.Min(),
		"max":             dp.Max(),
		"bucket_counts":   counts,
		"explicit_bounds": bounds,
	}
}

// extractExponentialHistogramValue extracts exponential histogram data point values.
func (e *httpMetricsExporter) extractExponentialHistogramValue(dp pmetric.ExponentialHistogramDataPoint) map[string]interface{} {
	// Convert exponential histogram buckets to a format suitable for JSON serialization
	convertBucketData := func(buckets pmetric.ExponentialHistogramDataPointBuckets) map[string]interface{} {
		offset := buckets.Offset()
		bucketCounts := buckets.BucketCounts()
		bucketLen := bucketCounts.Len()
		counts := make([]uint64, bucketLen)
		for i := 0; i < bucketLen; i++ {
			counts[i] = bucketCounts.At(i)
		}
		return map[string]interface{}{
			"offset": offset,
			"counts": counts,
		}
	}

	return map[string]interface{}{
		"count":          dp.Count(),
		"min":            dp.Min(),
		"max":            dp.Max(),
		"sum":            dp.Sum(),
		"scale":          dp.Scale(),
		"zero_count":     dp.ZeroCount(),
		"positive":       convertBucketData(dp.Positive()),
		"negative":       convertBucketData(dp.Negative()),
		"zero_threshold": dp.ZeroThreshold(),
	}
}
