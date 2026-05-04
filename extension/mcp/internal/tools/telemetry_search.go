// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type SearchTracesInput struct {
	ServiceName string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	SpanName    string `json:"span_name,omitempty" jsonschema:"Filter by span name (partial match)"`
	TraceID     string `json:"trace_id,omitempty" jsonschema:"Filter by trace ID (partial match)"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum number of spans to return,100"`
}

type SearchTracesOutput struct {
	SpanCount int      `json:"span_count"`
	TraceIDs  []string `json:"trace_ids"`
	Spans     []string `json:"spans"`
}

// RegisterSearchTraces registers the search_traces tool
func RegisterSearchTraces(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[SearchTracesInput, SearchTracesOutput](server, &mcp.Tool{
		Name:        "search_traces",
		Description: "Search traces by criteria (service name, span name, trace ID). Returns matching spans with details.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchTracesInput) (*mcp.CallToolResult, SearchTracesOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		traces := ext.GetRecentTraces(1000, 0) // Get a large batch to search
		spans := []string{}
		traceIDMap := make(map[string]bool)
		spanCount := 0

		for _, td := range traces {
			if spanCount >= limit {
				break
			}

			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, SearchTracesOutput{}, ctx.Err()
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

				// Filter by service name if specified
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

						// Filter by span name if specified (partial match)
						if input.SpanName != "" && !strings.Contains(strings.ToLower(spanName), strings.ToLower(input.SpanName)) {
							continue
						}

						// Filter by trace ID if specified (partial match)
						if input.TraceID != "" && !strings.Contains(strings.ToLower(traceID), strings.ToLower(input.TraceID)) {
							continue
						}

						spanCount++
						traceIDMap[traceID] = true
						spanSummary := fmt.Sprintf("trace_id=%s span_id=%s service=%s span=%s status=%s",
							traceID[:16]+"...",
							span.SpanID().String()[:8]+"...",
							serviceName,
							spanName,
							span.Status().Code().String())
						spans = append(spans, spanSummary)
					}
				}
			}
		}

		traceIDs := make([]string, 0, len(traceIDMap))
		for tid := range traceIDMap {
			traceIDs = append(traceIDs, tid)
		}

		return nil, SearchTracesOutput{
			SpanCount: spanCount,
			TraceIDs:  traceIDs,
			Spans:     spans,
		}, nil
	})
}

type SearchLogsInput struct {
	SeverityText string `json:"severity_text,omitempty" jsonschema:"Filter by severity (INFO, WARN, ERROR, etc.)"`
	Body         string `json:"body,omitempty" jsonschema:"Filter by log body (partial match)"`
	ServiceName  string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum number of logs to return,100"`
}

type SearchLogsOutput struct {
	LogCount int    `json:"log_count"`
	Markdown string `json:"markdown"`
}

// RegisterSearchLogs registers the search_logs tool
func RegisterSearchLogs(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[SearchLogsInput, SearchLogsOutput](server, &mcp.Tool{
		Name:        "search_logs",
		Description: "Search logs by criteria (severity, body text, service name). Returns matching log records.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchLogsInput) (*mcp.CallToolResult, SearchLogsOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		logs := ext.GetRecentLogs(1000, 0) // Get a large batch to search
		var sb strings.Builder
		logCount := 0

		// Table header
		sb.WriteString("| Time | Severity | Service | Body | TraceID | Attributes |\n")
		sb.WriteString("|------|----------|---------|------|---------|------------|\n")

		for _, ld := range logs {
			if logCount >= limit {
				break
			}

			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, SearchLogsOutput{}, ctx.Err()
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

				// Filter by service name if specified
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
						severityText := lr.SeverityText()
						body := lr.Body().AsString()

						// Filter by severity if specified
						if input.SeverityText != "" && !strings.EqualFold(severityText, input.SeverityText) {
							continue
						}

						// Filter by body text if specified (partial match)
						if input.Body != "" && !strings.Contains(strings.ToLower(body), strings.ToLower(input.Body)) {
							continue
						}

						logCount++

						// Format timestamp
						timestamp := time.Unix(0, int64(lr.Timestamp()))
						timeStr := timestamp.Format("15:04:05.000")

						// Get trace ID if present
						traceID := lr.TraceID().String()
						traceIDShort := "-"
						if traceID != "" && traceID != "00000000000000000000000000000000" {
							traceIDShort = traceID[:8] + "..."
						}

						// Format attributes
						attrs := formatAttributes(lr.Attributes())
						if attrs == "" {
							attrs = "-"
						} else if len(attrs) > 40 {
							attrs = attrs[:40] + "..."
						}

						// Truncate body
						bodyTrunc := truncateString(body, 50)

						sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
							timeStr,
							severityText,
							serviceName,
							bodyTrunc,
							traceIDShort,
							attrs))
					}
				}
			}
		}

		markdown := sb.String()
		if logCount == 0 {
			markdown = "No logs found matching the criteria"
		}

		return nil, SearchLogsOutput{
			LogCount: logCount,
			Markdown: markdown,
		}, nil
	})
}

type SearchMetricsInput struct {
	MetricName  string `json:"metric_name,omitempty" jsonschema:"Filter by metric name (partial match)"`
	ServiceName string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum number of metrics to return,100"`
}

type SearchMetricsOutput struct {
	MetricCount int    `json:"metric_count"`
	Markdown    string `json:"markdown"`
}

// RegisterSearchMetrics registers the search_metrics tool
func RegisterSearchMetrics(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[SearchMetricsInput, SearchMetricsOutput](server, &mcp.Tool{
		Name:        "search_metrics",
		Description: "Search metrics by criteria (metric name, service name). Returns matching metrics with details.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchMetricsInput) (*mcp.CallToolResult, SearchMetricsOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		metricsData := ext.GetRecentMetrics(1000, 0) // Get a large batch to search
		var sb strings.Builder
		metricCount := 0

		// Table header
		sb.WriteString("| Metric | Type | Service | Unit | Value | Attributes |\n")
		sb.WriteString("|--------|------|---------|------|-------|------------|\n")

		for _, md := range metricsData {
			if metricCount >= limit {
				break
			}

			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, SearchMetricsOutput{}, ctx.Err()
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

				// Filter by service name if specified
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

						// Filter by metric name if specified (partial match)
						if input.MetricName != "" && !strings.Contains(strings.ToLower(metricName), strings.ToLower(input.MetricName)) {
							continue
						}

						metricCount++

						// Extract value summary based on type
						valueStr := "-"
						attrStr := "-"

						switch metric.Type() {
						case pmetric.MetricTypeSum:
							sum := metric.Sum()
							if sum.DataPoints().Len() > 0 {
								dp := sum.DataPoints().At(0)
								valueStr = fmt.Sprintf("%.2f", dp.DoubleValue())
								attrStr = formatAttributes(dp.Attributes())
							}
						case pmetric.MetricTypeGauge:
							gauge := metric.Gauge()
							if gauge.DataPoints().Len() > 0 {
								dp := gauge.DataPoints().At(0)
								valueStr = fmt.Sprintf("%.2f", dp.DoubleValue())
								attrStr = formatAttributes(dp.Attributes())
							}
						case pmetric.MetricTypeHistogram:
							hist := metric.Histogram()
							if hist.DataPoints().Len() > 0 {
								dp := hist.DataPoints().At(0)
								valueStr = fmt.Sprintf("count=%d sum=%.2f", dp.Count(), dp.Sum())
								attrStr = formatAttributes(dp.Attributes())
							}
						case pmetric.MetricTypeSummary:
							summ := metric.Summary()
							if summ.DataPoints().Len() > 0 {
								dp := summ.DataPoints().At(0)
								valueStr = fmt.Sprintf("count=%d sum=%.2f", dp.Count(), dp.Sum())
								attrStr = formatAttributes(dp.Attributes())
							}
						}

						// Truncate attributes
						if len(attrStr) > 50 {
							attrStr = attrStr[:50] + "..."
						}
						if attrStr == "" {
							attrStr = "-"
						}

						sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
							metricName,
							metric.Type().String(),
							serviceName,
							metric.Unit(),
							valueStr,
							attrStr))
					}
				}
			}
		}

		markdown := sb.String()
		if metricCount == 0 {
			markdown = "No metrics found matching the criteria"
		}

		return nil, SearchMetricsOutput{
			MetricCount: metricCount,
			Markdown:    markdown,
		}, nil
	})
}

type GetTraceByIDInput struct {
	TraceID string `json:"trace_id" jsonschema:"Full trace ID to retrieve,required"`
}

type GetTraceByIDOutput struct {
	TraceID   string `json:"trace_id"`
	SpanCount int    `json:"span_count"`
	Markdown  string `json:"markdown"`
	Found     bool   `json:"found"`
}

// spanInfo holds span data for waterfall rendering
type spanInfo struct {
	spanID     string
	parentID   string
	name       string
	startTime  time.Time
	endTime    time.Time
	status     string
	kind       string
	attributes map[string]string
	children   []*spanInfo
}

// RegisterGetTraceByID registers the get_trace_by_id tool
func RegisterGetTraceByID(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetTraceByIDInput, GetTraceByIDOutput](server, &mcp.Tool{
		Name:        "get_trace_by_id",
		Description: "Get a specific trace by trace ID. Returns all spans for the trace.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetTraceByIDInput) (*mcp.CallToolResult, GetTraceByIDOutput, error) {
		if input.TraceID == "" {
			return nil, GetTraceByIDOutput{}, errors.New("trace_id is required")
		}

		traces := ext.GetRecentTraces(1000, 0) // Get all recent traces
		spanMap := make(map[string]*spanInfo)
		var traceStartTime time.Time
		found := false

		// Collect all spans for this trace
		for _, td := range traces {
			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, GetTraceByIDOutput{}, ctx.Err()
			}

			for i := 0; i < td.ResourceSpans().Len(); i++ {
				rs := td.ResourceSpans().At(i)
				for j := 0; j < rs.ScopeSpans().Len(); j++ {
					ss := rs.ScopeSpans().At(j)
					for k := 0; k < ss.Spans().Len(); k++ {
						span := ss.Spans().At(k)
						traceID := span.TraceID().String()

						// Match exact trace ID
						if traceID == input.TraceID {
							found = true
							info := extractSpanInfo(span)
							spanMap[info.spanID] = info

							// Track earliest start time as trace start
							if traceStartTime.IsZero() || info.startTime.Before(traceStartTime) {
								traceStartTime = info.startTime
							}
						}
					}
				}
			}
		}

		if !found {
			return nil, GetTraceByIDOutput{
				TraceID:   input.TraceID,
				SpanCount: 0,
				Markdown:  "Trace not found",
				Found:     false,
			}, nil
		}

		// Build tree structure
		rootSpans := buildSpanTree(spanMap)

		// Render as markdown waterfall
		markdown := renderTraceWaterfall(rootSpans, traceStartTime)

		return nil, GetTraceByIDOutput{
			TraceID:   input.TraceID,
			SpanCount: len(spanMap),
			Markdown:  markdown,
			Found:     true,
		}, nil
	})
}

type FindRelatedTelemetryInput struct {
	TraceID string `json:"trace_id,omitempty" jsonschema:"Trace ID to find related telemetry"`
	SpanID  string `json:"span_id,omitempty" jsonschema:"Span ID to find related telemetry"`
}

type FindRelatedTelemetryOutput struct {
	TraceID     string   `json:"trace_id,omitempty"`
	SpanCount   int      `json:"span_count"`
	LogCount    int      `json:"log_count"`
	MetricCount int      `json:"metric_count"`
	Spans       []string `json:"spans,omitempty"`
	Logs        []string `json:"logs,omitempty"`
	Metrics     []string `json:"metrics,omitempty"`
}

// RegisterFindRelatedTelemetry registers the find_related_telemetry tool
func RegisterFindRelatedTelemetry(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[FindRelatedTelemetryInput, FindRelatedTelemetryOutput](server, &mcp.Tool{
		Name:        "find_related_telemetry",
		Description: "Find related telemetry (logs, metrics) based on trace context. Correlates logs and metrics with trace/span IDs.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input FindRelatedTelemetryInput) (*mcp.CallToolResult, FindRelatedTelemetryOutput, error) {
		if input.TraceID == "" && input.SpanID == "" {
			return nil, FindRelatedTelemetryOutput{}, errors.New("either trace_id or span_id is required")
		}

		output := FindRelatedTelemetryOutput{
			TraceID: input.TraceID,
		}

		// Find related spans if trace ID is provided
		if input.TraceID != "" {
			traces := ext.GetRecentTraces(1000, 0)
			for _, td := range traces {
				// Check for context cancellation
				if ctx.Err() != nil {
					return nil, FindRelatedTelemetryOutput{}, ctx.Err()
				}

				for i := 0; i < td.ResourceSpans().Len(); i++ {
					rs := td.ResourceSpans().At(i)
					for j := 0; j < rs.ScopeSpans().Len(); j++ {
						ss := rs.ScopeSpans().At(j)
						for k := 0; k < ss.Spans().Len(); k++ {
							span := ss.Spans().At(k)
							if span.TraceID().String() == input.TraceID {
								output.SpanCount++
								if output.Spans == nil {
									output.Spans = []string{}
								}
								output.Spans = append(output.Spans, fmt.Sprintf("span_id=%s name=%s",
									span.SpanID().String(), span.Name()))
							}
						}
					}
				}
			}
		}

		// Find related logs
		logs := ext.GetRecentLogs(1000, 0)
		for _, ld := range logs {
			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, FindRelatedTelemetryOutput{}, ctx.Err()
			}

			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				rl := ld.ResourceLogs().At(i)
				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					sl := rl.ScopeLogs().At(j)
					for k := 0; k < sl.LogRecords().Len(); k++ {
						lr := sl.LogRecords().At(k)

						// Check if log has matching trace/span ID
						logTraceID := lr.TraceID().String()
						logSpanID := lr.SpanID().String()

						matched := false
						if input.TraceID != "" && logTraceID == input.TraceID {
							matched = true
						}
						if input.SpanID != "" && logSpanID == input.SpanID {
							matched = true
						}

						if matched {
							output.LogCount++
							if output.Logs == nil {
								output.Logs = []string{}
							}
							output.Logs = append(output.Logs, fmt.Sprintf("severity=%s body=%s",
								lr.SeverityText(), truncateString(lr.Body().AsString(), 60)))
						}
					}
				}
			}
		}

		// Note: Metrics typically don't have trace/span context in OTLP,
		// so we can't easily correlate them without exemplars
		// Leaving metric count at 0 for now

		return nil, output, nil
	})
}

// Helper function to truncate strings in a UTF-8 safe manner
func truncateString(s string, maxLen int) string {
	// Convert to runes to handle multi-byte UTF-8 characters correctly
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// extractSpanInfo extracts relevant span information for waterfall rendering
func extractSpanInfo(span ptrace.Span) *spanInfo {
	info := &spanInfo{
		spanID:     span.SpanID().String(),
		parentID:   span.ParentSpanID().String(),
		name:       span.Name(),
		startTime:  time.Unix(0, int64(span.StartTimestamp())),
		endTime:    time.Unix(0, int64(span.EndTimestamp())),
		status:     span.Status().Code().String(),
		kind:       span.Kind().String(),
		attributes: make(map[string]string),
		children:   []*spanInfo{},
	}

	// Extract key attributes (limit to avoid overwhelming output)
	span.Attributes().Range(func(k string, v pcommon.Value) bool {
		if len(info.attributes) < 5 { // Limit to 5 key attributes
			info.attributes[k] = v.AsString()
		}
		return true
	})

	return info
}

// buildSpanTree builds a tree structure from flat span map
func buildSpanTree(spanMap map[string]*spanInfo) []*spanInfo {
	roots := []*spanInfo{}

	// First pass: link children to parents
	for _, span := range spanMap {
		if span.parentID == "" || span.parentID == "0000000000000000" {
			// Root span (no parent)
			roots = append(roots, span)
		} else {
			// Child span - add to parent's children
			if parent, ok := spanMap[span.parentID]; ok {
				parent.children = append(parent.children, span)
			} else {
				// Parent not found, treat as root
				roots = append(roots, span)
			}
		}
	}

	// Sort roots by start time
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].startTime.Before(roots[j].startTime)
	})

	// Sort children by start time recursively
	for _, root := range roots {
		sortChildren(root)
	}

	return roots
}

// sortChildren recursively sorts children by start time
func sortChildren(span *spanInfo) {
	sort.Slice(span.children, func(i, j int) bool {
		return span.children[i].startTime.Before(span.children[j].startTime)
	})

	for _, child := range span.children {
		sortChildren(child)
	}
}

// renderTraceWaterfall renders spans as a markdown table with tree structure
func renderTraceWaterfall(roots []*spanInfo, traceStart time.Time) string {
	var sb strings.Builder

	// Table header
	sb.WriteString("| Span | ID | Duration | Start | Status | Attributes |\n")
	sb.WriteString("|------|-----|----------|-------|--------|------------|\n")

	// Render each root and its children
	for _, root := range roots {
		renderSpanRow(&sb, root, traceStart, "", true)
	}

	return sb.String()
}

// renderSpanRow renders a single span row with tree formatting
// prefix contains only the indentation (│ and spaces from ancestors)
// isLast indicates if this is the last child of its parent
func renderSpanRow(sb *strings.Builder, span *spanInfo, traceStart time.Time, prefix string, isLast bool) {
	// Calculate timing
	duration := span.endTime.Sub(span.startTime)
	startOffset := span.startTime.Sub(traceStart)

	// Format duration
	durationStr := formatDuration(duration)
	startStr := fmt.Sprintf("%.3fs", startOffset.Seconds())

	// Truncate span ID for display
	spanIDShort := span.spanID
	if len(spanIDShort) > 8 {
		spanIDShort = spanIDShort[:8]
	}

	// Format attributes
	attrs := formatAttributesMap(span.attributes, 50)

	// Build the tree character for this span
	treeChar := ""
	if prefix != "" {
		if isLast {
			treeChar = "└─ "
		} else {
			treeChar = "├─ "
		}
	}

	// Write row
	fmt.Fprintf(sb, "| %s%s%s | %s | %s | %s | %s | %s |\n",
		prefix,
		treeChar,
		span.name,
		spanIDShort,
		durationStr,
		startStr,
		span.status,
		attrs)

	// Render children with updated indentation
	for i, child := range span.children {
		isChildLast := i == len(span.children)-1

		// Build child's prefix (indentation only, no tree character)
		childPrefix := prefix
		if prefix != "" || len(span.children) > 0 {
			// Add continuation or space based on whether this span has more siblings
			if isLast {
				childPrefix += "   "
			} else {
				childPrefix += "│  "
			}
		}

		renderSpanRow(sb, child, traceStart, childPrefix, isChildLast)
	}
}

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1000)
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// formatAttributesMap formats attribute map as compact string
func formatAttributesMap(attrs map[string]string, maxLen int) string {
	if len(attrs) == 0 {
		return "-"
	}

	var parts []string
	for k, v := range attrs {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}

	// Sort for consistent output
	sort.Strings(parts)

	result := strings.Join(parts, " ")
	if len(result) > maxLen {
		result = result[:maxLen] + "..."
	}

	return result
}
