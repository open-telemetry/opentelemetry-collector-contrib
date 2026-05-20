// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type TracesInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"Maximum number of trace batches to return,10"`
	Offset int `json:"offset,omitempty" jsonschema:"Number of trace batches to skip,0"`
}

type TracesOutput struct {
	Count  int      `json:"count"`
	Traces []string `json:"traces"`
	CSV    string   `json:"csv"`
}

// RegisterGetRecentTraces registers the get_recent_traces tool
func RegisterGetRecentTraces(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recent_traces",
		Description: "Get recent traces from the circular buffer (CSV format)",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input TracesInput) (*mcp.CallToolResult, TracesOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		limit := input.Limit
		if limit == 0 {
			limit = 10
		}

		traces := ext.GetRecentTraces(limit, input.Offset)

		// Build CSV output using encoding/csv
		var buf strings.Builder
		w := csv.NewWriter(&buf)

		// Write header
		if err := w.Write([]string{"trace_id", "span_id", "parent_span_id", "span_name", "service_name", "start_time", "end_time", "duration_ms", "status_code", "span_kind"}); err != nil {
			return nil, TracesOutput{}, fmt.Errorf("failed to write CSV header: %w", err)
		}

		summaries := []string{}
		spanCount := 0

		for _, td := range traces {
			for i := 0; i < td.ResourceSpans().Len(); i++ {
				rs := td.ResourceSpans().At(i)
				serviceName := "unknown"
				if sn, ok := rs.Resource().Attributes().Get("service.name"); ok {
					serviceName = sn.AsString()
				}

				for j := 0; j < rs.ScopeSpans().Len(); j++ {
					ss := rs.ScopeSpans().At(j)
					for k := 0; k < ss.Spans().Len(); k++ {
						span := ss.Spans().At(k)
						spanCount++

						traceID := span.TraceID().String()
						spanID := span.SpanID().String()
						parentSpanID := span.ParentSpanID().String()
						spanName := span.Name()
						startTime := time.Unix(0, int64(span.StartTimestamp())).Format(time.RFC3339)
						endTime := time.Unix(0, int64(span.EndTimestamp())).Format(time.RFC3339)
						durationMs := fmt.Sprintf("%.2f", float64(span.EndTimestamp()-span.StartTimestamp())/1e6)
						statusCode := span.Status().Code().String()
						spanKind := span.Kind().String()

						if err := w.Write([]string{traceID, spanID, parentSpanID, spanName, serviceName, startTime, endTime, durationMs, statusCode, spanKind}); err != nil {
							return nil, TracesOutput{}, fmt.Errorf("failed to write CSV row: %w", err)
						}
					}
				}
			}
		}

		w.Flush()
		if err := w.Error(); err != nil {
			return nil, TracesOutput{}, fmt.Errorf("CSV writer error: %w", err)
		}

		if spanCount > 0 {
			summaries = append(summaries, fmt.Sprintf("Total spans: %d across %d batches", spanCount, len(traces)))
		}

		return nil, TracesOutput{
			Count:  len(traces),
			Traces: summaries,
			CSV:    buf.String(),
		}, nil
	})
}

type MetricsInput struct {
	MetricName string `json:"metric_name,omitempty" jsonschema:"Optional metric name to filter by. If omitted returns list of all metric names"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of metric batches to search,10"`
}

type MetricSummary struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Unit  string `json:"unit"`
	Count int    `json:"count"`
}

type MetricDataPoint struct {
	Value      string            `json:"value"`
	Timestamp  string            `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type MetricsOutput struct {
	// If no metric_name specified, returns list of available metrics
	AvailableMetrics []MetricSummary `json:"available_metrics,omitempty"`

	// If metric_name specified, returns data for that metric
	MetricName    string            `json:"metric_name,omitempty"`
	MetricType    string            `json:"metric_type,omitempty"`
	Unit          string            `json:"unit,omitempty"`
	DataPoints    []MetricDataPoint `json:"data_points,omitempty"`
	ResourceAttrs map[string]string `json:"resource_attrs,omitempty"`
}

// RegisterGetRecentMetrics registers the get_recent_metrics tool
func RegisterGetRecentMetrics(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recent_metrics",
		Description: "Get recent metrics from the circular buffer. Without metric_name returns list of available metrics. With metric_name returns latest values for that metric.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input MetricsInput) (*mcp.CallToolResult, MetricsOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		limit := input.Limit
		if limit == 0 {
			limit = 10
		}

		metrics := ext.GetRecentMetrics(limit, 0)

		// If no metric name specified, return list of available metrics
		if input.MetricName == "" {
			metricMap := make(map[string]*MetricSummary)

			for _, md := range metrics {
				for i := 0; i < md.ResourceMetrics().Len(); i++ {
					rm := md.ResourceMetrics().At(i)
					for j := 0; j < rm.ScopeMetrics().Len(); j++ {
						sm := rm.ScopeMetrics().At(j)
						for k := 0; k < sm.Metrics().Len(); k++ {
							metric := sm.Metrics().At(k)
							name := metric.Name()

							if summary, exists := metricMap[name]; exists {
								summary.Count++
							} else {
								metricMap[name] = &MetricSummary{
									Name:  name,
									Type:  metric.Type().String(),
									Unit:  metric.Unit(),
									Count: 1,
								}
							}
						}
					}
				}
			}

			summaries := make([]MetricSummary, 0, len(metricMap))
			for _, summary := range metricMap {
				summaries = append(summaries, *summary)
			}

			return nil, MetricsOutput{
				AvailableMetrics: summaries,
			}, nil
		}

		// If metric name specified, return data for that metric
		var dataPoints []MetricDataPoint
		var metricType, unit string
		var resourceAttrs map[string]string

		for _, md := range metrics {
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				rm := md.ResourceMetrics().At(i)

				// Capture resource attributes from first match
				if resourceAttrs == nil {
					resourceAttrs = make(map[string]string)
					rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
						resourceAttrs[k] = v.AsString()
						return true
					})
				}

				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						if metric.Name() != input.MetricName {
							continue
						}

						metricType = metric.Type().String()
						unit = metric.Unit()

						// Extract data points based on type
						switch metric.Type() {
						case pmetric.MetricTypeGauge:
							dps := metric.Gauge().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								dp := dps.At(l)
								attrs := make(map[string]string)
								dp.Attributes().Range(func(k string, v pcommon.Value) bool {
									attrs[k] = v.AsString()
									return true
								})
								dataPoints = append(dataPoints, MetricDataPoint{
									Value:      formatNumberDataPoint(dp),
									Timestamp:  time.Unix(0, int64(dp.Timestamp())).Format(time.RFC3339),
									Attributes: attrs,
								})
							}
						case pmetric.MetricTypeSum:
							dps := metric.Sum().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								dp := dps.At(l)
								attrs := make(map[string]string)
								dp.Attributes().Range(func(k string, v pcommon.Value) bool {
									attrs[k] = v.AsString()
									return true
								})
								dataPoints = append(dataPoints, MetricDataPoint{
									Value:      formatNumberDataPoint(dp),
									Timestamp:  time.Unix(0, int64(dp.Timestamp())).Format(time.RFC3339),
									Attributes: attrs,
								})
							}
						case pmetric.MetricTypeHistogram:
							dps := metric.Histogram().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								dp := dps.At(l)
								attrs := make(map[string]string)
								dp.Attributes().Range(func(k string, v pcommon.Value) bool {
									attrs[k] = v.AsString()
									return true
								})
								dataPoints = append(dataPoints, MetricDataPoint{
									Value:      fmt.Sprintf("count=%d,sum=%.2f", dp.Count(), dp.Sum()),
									Timestamp:  time.Unix(0, int64(dp.Timestamp())).Format(time.RFC3339),
									Attributes: attrs,
								})
							}
						case pmetric.MetricTypeSummary:
							dps := metric.Summary().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								dp := dps.At(l)
								attrs := make(map[string]string)
								dp.Attributes().Range(func(k string, v pcommon.Value) bool {
									attrs[k] = v.AsString()
									return true
								})
								dataPoints = append(dataPoints, MetricDataPoint{
									Value:      fmt.Sprintf("count=%d,sum=%.2f", dp.Count(), dp.Sum()),
									Timestamp:  time.Unix(0, int64(dp.Timestamp())).Format(time.RFC3339),
									Attributes: attrs,
								})
							}
						}
					}
				}
			}
		}

		// Return only the last data point by default for compactness
		if len(dataPoints) > 1 {
			dataPoints = dataPoints[len(dataPoints)-1:]
		}

		return nil, MetricsOutput{
			MetricName:    input.MetricName,
			MetricType:    metricType,
			Unit:          unit,
			DataPoints:    dataPoints,
			ResourceAttrs: resourceAttrs,
		}, nil
	})
}

type LogsInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"Maximum number of log batches to return,10"`
	Offset int `json:"offset,omitempty" jsonschema:"Number of log batches to skip,0"`
}

type LogsOutput struct {
	Count int      `json:"count"`
	Logs  []string `json:"logs"`
	CSV   string   `json:"csv"`
}

// RegisterGetRecentLogs registers the get_recent_logs tool
func RegisterGetRecentLogs(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recent_logs",
		Description: "Get recent logs from the circular buffer (CSV format)",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input LogsInput) (*mcp.CallToolResult, LogsOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		limit := input.Limit
		if limit == 0 {
			limit = 10
		}

		logs := ext.GetRecentLogs(limit, input.Offset)

		// Build CSV output using encoding/csv
		var buf strings.Builder
		w := csv.NewWriter(&buf)

		// Write header
		if err := w.Write([]string{"timestamp", "severity", "body", "resource_attrs", "log_attrs"}); err != nil {
			return nil, LogsOutput{}, fmt.Errorf("failed to write CSV header: %w", err)
		}

		summaries := []string{}
		logCount := 0

		for _, ld := range logs {
			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				rl := ld.ResourceLogs().At(i)
				resourceAttrs := formatAttributes(rl.Resource().Attributes())

				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					sl := rl.ScopeLogs().At(j)
					for k := 0; k < sl.LogRecords().Len(); k++ {
						logRecord := sl.LogRecords().At(k)
						logCount++

						timestamp := time.Unix(0, int64(logRecord.Timestamp())).Format(time.RFC3339)
						severity := logRecord.SeverityText()
						body := logRecord.Body().AsString()
						logAttrs := formatAttributes(logRecord.Attributes())

						// encoding/csv handles escaping automatically
						if err := w.Write([]string{timestamp, severity, body, resourceAttrs, logAttrs}); err != nil {
							return nil, LogsOutput{}, fmt.Errorf("failed to write CSV row: %w", err)
						}
					}
				}
			}
		}

		w.Flush()
		if err := w.Error(); err != nil {
			return nil, LogsOutput{}, fmt.Errorf("CSV writer error: %w", err)
		}

		if logCount > 0 {
			summaries = append(summaries, fmt.Sprintf("Total logs: %d across %d batches", logCount, len(logs)))
		}

		return nil, LogsOutput{
			Count: len(logs),
			Logs:  summaries,
			CSV:   buf.String(),
		}, nil
	})
}

type TelemetrySummaryOutput struct {
	Traces  BufferInfo `json:"traces"`
	Metrics BufferInfo `json:"metrics"`
	Logs    BufferInfo `json:"logs"`
}

type BufferInfo struct {
	Count    int `json:"count"`
	Capacity int `json:"capacity"`
}

// RegisterGetTelemetrySummary registers the get_telemetry_summary tool
func RegisterGetTelemetrySummary(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_telemetry_summary",
		Description: "Get statistics about buffered telemetry",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input any) (*mcp.CallToolResult, TelemetrySummaryOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		stats := ext.GetBufferStats()

		return nil, TelemetrySummaryOutput{
			Traces: BufferInfo{
				Count:    stats.TracesCount,
				Capacity: stats.TracesCapacity,
			},
			Metrics: BufferInfo{
				Count:    stats.MetricsCount,
				Capacity: stats.MetricsCapacity,
			},
			Logs: BufferInfo{
				Count:    stats.LogsCount,
				Capacity: stats.LogsCapacity,
			},
		}, nil
	})
}

// Helper functions
func formatAttributes(attrs pcommon.Map) string {
	if attrs.Len() == 0 {
		return ""
	}

	var parts []string
	attrs.Range(func(k string, v pcommon.Value) bool {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v.AsString()))
		return true
	})
	return strings.Join(parts, ";")
}

func formatNumberDataPoint(dp pmetric.NumberDataPoint) string {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return fmt.Sprintf("%d", dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		return fmt.Sprintf("%.2f", dp.DoubleValue())
	default:
		return "0"
	}
}
