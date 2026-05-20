// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// QueryTracesInput provides flexible filtering for trace queries
type QueryTracesInput struct {
	ServiceName string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	SpanName    string `json:"span_name,omitempty" jsonschema:"Filter by span name (partial match)"`
	TraceID     string `json:"trace_id,omitempty" jsonschema:"Filter by trace ID (partial match)"`
	Status      string `json:"status,omitempty" jsonschema:"Filter by status (Ok, Error, Unset)"`
	MinDuration string `json:"min_duration,omitempty" jsonschema:"Minimum span duration (e.g. '100ms', '1s')"`
	MaxDuration string `json:"max_duration,omitempty" jsonschema:"Maximum span duration (e.g. '5s', '1m')"`
	Detailed    bool   `json:"detailed,omitempty" jsonschema:"Return detailed information for each span,false"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum number of spans to return,100"`
	Offset      int    `json:"offset,omitempty" jsonschema:"Number of spans to skip,0"`
}

type QueryTracesOutput struct {
	SpanCount int    `json:"span_count"`
	Markdown  string `json:"markdown"`
}

// RegisterQueryTraces registers the query_traces tool
func RegisterQueryTraces(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[QueryTracesInput, QueryTracesOutput](server, &mcp.Tool{
		Name:        "query_traces",
		Description: "Query traces with flexible filtering. Returns matching spans in table format or detailed view.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input QueryTracesInput) (*mcp.CallToolResult, QueryTracesOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		var minDuration, maxDuration time.Duration
		var err error
		if input.MinDuration != "" {
			if minDuration, err = time.ParseDuration(input.MinDuration); err != nil {
				minDuration = 0
			}
		}
		if input.MaxDuration != "" {
			if maxDuration, err = time.ParseDuration(input.MaxDuration); err != nil {
				maxDuration = 0
			}
		}

		traces := ext.GetRecentTraces(10000, 0)
		var sb strings.Builder
		writer := &TraceWriter{}
		spanCount := 0
		skipped := 0

		if !input.Detailed {
			sb.WriteString("| Span | ID | Duration | Service | Status | Attributes |\n")
			sb.WriteString("|------|-----|----------|---------|--------|------------|\n")
		}

		for _, td := range traces {
			if spanCount >= limit {
				break
			}

			if ctx.Err() != nil {
				return nil, QueryTracesOutput{}, ctx.Err()
			}

			for i := 0; i < td.ResourceSpans().Len(); i++ {
				if spanCount >= limit {
					break
				}

				rs := td.ResourceSpans().At(i)
				serviceName := "unknown"
				if sn, ok := rs.Resource().Attributes().Get("service.name"); ok {
					serviceName = sn.AsString()
				}

				if input.ServiceName != "" && serviceName != input.ServiceName {
					continue
				}

				for j := 0; j < rs.ScopeSpans().Len(); j++ {
					if spanCount >= limit {
						break
					}

					ss := rs.ScopeSpans().At(j)
					for k := 0; k < ss.Spans().Len(); k++ {
						if spanCount >= limit {
							break
						}

						span := ss.Spans().At(k)
						spanName := span.Name()
						traceID := span.TraceID().String()

						if input.SpanName != "" && !strings.Contains(strings.ToLower(spanName), strings.ToLower(input.SpanName)) {
							continue
						}

						if input.TraceID != "" && !strings.Contains(strings.ToLower(traceID), strings.ToLower(input.TraceID)) {
							continue
						}

						if input.Status != "" && span.Status().Code().String() != input.Status {
							continue
						}

						startTime := time.Unix(0, int64(span.StartTimestamp()))
						endTime := time.Unix(0, int64(span.EndTimestamp()))
						duration := endTime.Sub(startTime)

						if minDuration > 0 && duration < minDuration {
							continue
						}

						if maxDuration > 0 && duration > maxDuration {
							continue
						}

						if skipped < input.Offset {
							skipped++
							continue
						}

						spanCount++

						if input.Detailed {
							writer.WriteSpanDetailed(&sb, span, serviceName, rs.Resource().Attributes())
						} else {
							info := extractSpanInfo(span)
							spanIDShort := info.spanID
							if len(spanIDShort) > 8 {
								spanIDShort = spanIDShort[:8]
							}
							durationStr := formatDuration(duration)
							attrs := formatAttributesMap(info.attributes, 40)

							sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
								spanName, spanIDShort, durationStr, serviceName, info.status, attrs))
						}
					}
				}
			}
		}

		markdown := sb.String()
		if spanCount == 0 {
			markdown = "No spans found matching the criteria"
		}

		return nil, QueryTracesOutput{
			SpanCount: spanCount,
			Markdown:  markdown,
		}, nil
	})
}

// QueryLogsInput provides flexible filtering for log queries
type QueryLogsInput struct {
	SeverityText string `json:"severity_text,omitempty" jsonschema:"Filter by severity (INFO, WARN, ERROR, etc.)"`
	Body         string `json:"body,omitempty" jsonschema:"Filter by log body (partial match)"`
	ServiceName  string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	TraceID      string `json:"trace_id,omitempty" jsonschema:"Filter by trace ID (partial match)"`
	SpanID       string `json:"span_id,omitempty" jsonschema:"Filter by span ID (partial match)"`
	Detailed     bool   `json:"detailed,omitempty" jsonschema:"Return detailed information for each log,false"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum number of logs to return,100"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Number of logs to skip,0"`
}

type QueryLogsOutput struct {
	LogCount int    `json:"log_count"`
	Markdown string `json:"markdown"`
}

// RegisterQueryLogs registers the query_logs tool
func RegisterQueryLogs(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[QueryLogsInput, QueryLogsOutput](server, &mcp.Tool{
		Name:        "query_logs",
		Description: "Query logs with flexible filtering. Returns matching logs in table format or detailed view.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input QueryLogsInput) (*mcp.CallToolResult, QueryLogsOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		logs := ext.GetRecentLogs(10000, 0)
		var sb strings.Builder
		writer := &LogWriter{}
		logCount := 0
		skipped := 0

		if !input.Detailed {
			sb.WriteString("| Time | Severity | Service | Body | TraceID | Attributes |\n")
			sb.WriteString("|------|----------|---------|------|---------|------------|\n")
		}

		for _, ld := range logs {
			if logCount >= limit {
				break
			}

			if ctx.Err() != nil {
				return nil, QueryLogsOutput{}, ctx.Err()
			}

			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				if logCount >= limit {
					break
				}

				rl := ld.ResourceLogs().At(i)
				serviceName := "unknown"
				if sn, ok := rl.Resource().Attributes().Get("service.name"); ok {
					serviceName = sn.AsString()
				}

				if input.ServiceName != "" && serviceName != input.ServiceName {
					continue
				}

				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					if logCount >= limit {
						break
					}

					sl := rl.ScopeLogs().At(j)
					for k := 0; k < sl.LogRecords().Len(); k++ {
						if logCount >= limit {
							break
						}

						lr := sl.LogRecords().At(k)

						if input.SeverityText != "" && !strings.EqualFold(lr.SeverityText(), input.SeverityText) {
							continue
						}

						body := lr.Body().AsString()
						if input.Body != "" && !strings.Contains(strings.ToLower(body), strings.ToLower(input.Body)) {
							continue
						}

						traceID := lr.TraceID().String()
						if input.TraceID != "" && !strings.Contains(strings.ToLower(traceID), strings.ToLower(input.TraceID)) {
							continue
						}

						spanID := lr.SpanID().String()
						if input.SpanID != "" && !strings.Contains(strings.ToLower(spanID), strings.ToLower(input.SpanID)) {
							continue
						}

						if skipped < input.Offset {
							skipped++
							continue
						}

						logCount++

						if input.Detailed {
							writer.WriteLogDetailed(&sb, lr, serviceName, rl.Resource().Attributes())
						} else {
							writer.WriteLogSummary(&sb, lr, serviceName)
						}
					}
				}
			}
		}

		markdown := sb.String()
		if logCount == 0 {
			markdown = "No logs found matching the criteria"
		}

		return nil, QueryLogsOutput{
			LogCount: logCount,
			Markdown: markdown,
		}, nil
	})
}

// QueryMetricsInput provides flexible filtering for metric queries
type QueryMetricsInput struct {
	MetricName  string `json:"metric_name,omitempty" jsonschema:"Filter by metric name (partial match)"`
	ServiceName string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	MetricType  string `json:"metric_type,omitempty" jsonschema:"Filter by metric type (Sum, Gauge, Histogram, Summary)"`
	Detailed    bool   `json:"detailed,omitempty" jsonschema:"Return detailed information for each metric,false"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum number of metrics to return,100"`
	Offset      int    `json:"offset,omitempty" jsonschema:"Number of metrics to skip,0"`
}

type QueryMetricsOutput struct {
	MetricCount int    `json:"metric_count"`
	Markdown    string `json:"markdown"`
}

// RegisterQueryMetrics registers the query_metrics tool
func RegisterQueryMetrics(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[QueryMetricsInput, QueryMetricsOutput](server, &mcp.Tool{
		Name:        "query_metrics",
		Description: "Query metrics with flexible filtering. Returns matching metrics in table format or detailed view.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input QueryMetricsInput) (*mcp.CallToolResult, QueryMetricsOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		metricsData := ext.GetRecentMetrics(10000, 0)
		var sb strings.Builder
		writer := &MetricWriter{}
		metricCount := 0
		skipped := 0

		if !input.Detailed {
			sb.WriteString("| Metric | Type | Service | Unit | Value | Attributes |\n")
			sb.WriteString("|--------|------|---------|------|-------|------------|\n")
		}

		for _, md := range metricsData {
			if metricCount >= limit {
				break
			}

			if ctx.Err() != nil {
				return nil, QueryMetricsOutput{}, ctx.Err()
			}

			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				if metricCount >= limit {
					break
				}

				rm := md.ResourceMetrics().At(i)
				serviceName := "unknown"
				if sn, ok := rm.Resource().Attributes().Get("service.name"); ok {
					serviceName = sn.AsString()
				}

				if input.ServiceName != "" && serviceName != input.ServiceName {
					continue
				}

				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					if metricCount >= limit {
						break
					}

					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						if metricCount >= limit {
							break
						}

						metric := sm.Metrics().At(k)
						metricName := metric.Name()

						if input.MetricName != "" && !strings.Contains(strings.ToLower(metricName), strings.ToLower(input.MetricName)) {
							continue
						}

						if input.MetricType != "" && metric.Type().String() != input.MetricType {
							continue
						}

						if skipped < input.Offset {
							skipped++
							continue
						}

						metricCount++

						if input.Detailed {
							writer.WriteMetricDetailed(&sb, metric, serviceName, rm.Resource().Attributes())
						} else {
							writer.WriteMetricSummary(&sb, metric, serviceName)
						}
					}
				}
			}
		}

		markdown := sb.String()
		if metricCount == 0 {
			markdown = "No metrics found matching the criteria"
		}

		return nil, QueryMetricsOutput{
			MetricCount: metricCount,
			Markdown:    markdown,
		}, nil
	})
}
