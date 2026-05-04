// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TraceWriter formats trace data in various output modes
type TraceWriter struct {
	traceStart time.Time
}

// WriteSpanSummary writes a single span as a table row
func (w *TraceWriter) WriteSpanSummary(sb *strings.Builder, span ptrace.Span, _, prefix string, isLast bool) {
	info := extractSpanInfo(span)

	duration := info.endTime.Sub(info.startTime)
	startOffset := info.startTime.Sub(w.traceStart)

	durationStr := formatDuration(duration)
	startStr := fmt.Sprintf("%.3fs", startOffset.Seconds())
	spanIDShort := info.spanID
	if len(spanIDShort) > 8 {
		spanIDShort = spanIDShort[:8]
	}
	attrs := formatAttributesMap(info.attributes, 50)

	treeChar := ""
	if prefix != "" {
		if isLast {
			treeChar = "└─ "
		} else {
			treeChar = "├─ "
		}
	}

	fmt.Fprintf(sb, "| %s%s%s | %s | %s | %s | %s | %s |\n",
		prefix, treeChar, info.name, spanIDShort, durationStr, startStr, info.status, attrs)
}

// WriteSpanDetailed writes full details of a span in markdown
func (*TraceWriter) WriteSpanDetailed(sb *strings.Builder, span ptrace.Span, _ string, resourceAttrs pcommon.Map) {
	fmt.Fprintf(sb, "## Span: %s\n\n", span.Name())
	fmt.Fprintf(sb, "**Trace ID:** `%s`\n\n", span.TraceID().String())
	fmt.Fprintf(sb, "**Span ID:** `%s`\n\n", span.SpanID().String())
	fmt.Fprintf(sb, "**Parent Span ID:** `%s`\n\n", span.ParentSpanID().String())
	fmt.Fprintf(sb, "**Kind:** %s\n\n", span.Kind().String())
	fmt.Fprintf(sb, "**Status:** %s", span.Status().Code().String())
	if span.Status().Message() != "" {
		fmt.Fprintf(sb, " - %s", span.Status().Message())
	}
	sb.WriteString("\n\n")

	startTime := time.Unix(0, int64(span.StartTimestamp()))
	endTime := time.Unix(0, int64(span.EndTimestamp()))
	duration := endTime.Sub(startTime)
	fmt.Fprintf(sb, "**Start:** %s\n\n", startTime.Format(time.RFC3339Nano))
	fmt.Fprintf(sb, "**End:** %s\n\n", endTime.Format(time.RFC3339Nano))
	fmt.Fprintf(sb, "**Duration:** %s\n\n", formatDuration(duration))

	if span.Attributes().Len() > 0 {
		sb.WriteString("### Span Attributes\n\n")
		sb.WriteString("| Key | Value |\n")
		sb.WriteString("|-----|-------|\n")
		span.Attributes().Range(func(k string, v pcommon.Value) bool {
			fmt.Fprintf(sb, "| %s | %s |\n", k, v.AsString())
			return true
		})
		sb.WriteString("\n")
	}

	if resourceAttrs.Len() > 0 {
		sb.WriteString("### Resource Attributes\n\n")
		sb.WriteString("| Key | Value |\n")
		sb.WriteString("|-----|-------|\n")
		resourceAttrs.Range(func(k string, v pcommon.Value) bool {
			fmt.Fprintf(sb, "| %s | %s |\n", k, v.AsString())
			return true
		})
		sb.WriteString("\n")
	}

	if span.Events().Len() > 0 {
		sb.WriteString("### Events\n\n")
		sb.WriteString("| Time | Name | Attributes |\n")
		sb.WriteString("|------|------|------------|\n")
		for i := 0; i < span.Events().Len(); i++ {
			event := span.Events().At(i)
			eventTime := time.Unix(0, int64(event.Timestamp()))
			attrs := formatAttributes(event.Attributes())
			if attrs == "" {
				attrs = "-"
			}
			fmt.Fprintf(sb, "| %s | %s | %s |\n",
				eventTime.Format("15:04:05.000"), event.Name(), attrs)
		}
		sb.WriteString("\n")
	}

	if span.Links().Len() > 0 {
		sb.WriteString("### Links\n\n")
		sb.WriteString("| Trace ID | Span ID | Attributes |\n")
		sb.WriteString("|----------|---------|------------|\n")
		for i := 0; i < span.Links().Len(); i++ {
			link := span.Links().At(i)
			attrs := formatAttributes(link.Attributes())
			if attrs == "" {
				attrs = "-"
			}
			fmt.Fprintf(sb, "| %s | %s | %s |\n",
				link.TraceID().String()[:16]+"...",
				link.SpanID().String()[:8]+"...",
				attrs)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")
}

// LogWriter formats log data in various output modes
type LogWriter struct{}

// WriteLogSummary writes a single log as a table row
func (*LogWriter) WriteLogSummary(sb *strings.Builder, lr plog.LogRecord, serviceName string) {
	timestamp := time.Unix(0, int64(lr.Timestamp()))
	timeStr := timestamp.Format("15:04:05.000")

	traceID := lr.TraceID().String()
	traceIDShort := "-"
	if traceID != "" && traceID != "00000000000000000000000000000000" {
		traceIDShort = traceID[:8] + "..."
	}

	attrs := formatAttributes(lr.Attributes())
	if attrs == "" {
		attrs = "-"
	} else if len(attrs) > 40 {
		attrs = attrs[:40] + "..."
	}

	body := truncateString(lr.Body().AsString(), 50)

	fmt.Fprintf(sb, "| %s | %s | %s | %s | %s | %s |\n",
		timeStr, lr.SeverityText(), serviceName, body, traceIDShort, attrs)
}

// WriteLogDetailed writes full details of a log in markdown
func (*LogWriter) WriteLogDetailed(sb *strings.Builder, lr plog.LogRecord, serviceName string, resourceAttrs pcommon.Map) {
	timestamp := time.Unix(0, int64(lr.Timestamp()))

	fmt.Fprintf(sb, "## Log Entry: %s\n\n", lr.SeverityText())
	fmt.Fprintf(sb, "**Timestamp:** %s\n\n", timestamp.Format(time.RFC3339Nano))
	fmt.Fprintf(sb, "**Severity:** %s (%d)\n\n", lr.SeverityText(), lr.SeverityNumber())
	fmt.Fprintf(sb, "**Service:** %s\n\n", serviceName)

	traceID := lr.TraceID().String()
	spanID := lr.SpanID().String()
	if traceID != "" && traceID != "00000000000000000000000000000000" {
		fmt.Fprintf(sb, "**Trace ID:** `%s`\n\n", traceID)
		fmt.Fprintf(sb, "**Span ID:** `%s`\n\n", spanID)
	}

	sb.WriteString("### Body\n\n")
	fmt.Fprintf(sb, "```\n%s\n```\n\n", lr.Body().AsString())

	if lr.Attributes().Len() > 0 {
		sb.WriteString("### Log Attributes\n\n")
		sb.WriteString("| Key | Value |\n")
		sb.WriteString("|-----|-------|\n")
		lr.Attributes().Range(func(k string, v pcommon.Value) bool {
			fmt.Fprintf(sb, "| %s | %s |\n", k, v.AsString())
			return true
		})
		sb.WriteString("\n")
	}

	if resourceAttrs.Len() > 0 {
		sb.WriteString("### Resource Attributes\n\n")
		sb.WriteString("| Key | Value |\n")
		sb.WriteString("|-----|-------|\n")
		resourceAttrs.Range(func(k string, v pcommon.Value) bool {
			fmt.Fprintf(sb, "| %s | %s |\n", k, v.AsString())
			return true
		})
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")
}

// MetricWriter formats metric data in various output modes
type MetricWriter struct{}

// WriteMetricSummary writes a single metric as a table row
func (*MetricWriter) WriteMetricSummary(sb *strings.Builder, metric pmetric.Metric, serviceName string) {
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

	if len(attrStr) > 50 {
		attrStr = attrStr[:50] + "..."
	}
	if attrStr == "" {
		attrStr = "-"
	}

	fmt.Fprintf(sb, "| %s | %s | %s | %s | %s | %s |\n",
		metric.Name(), metric.Type().String(), serviceName, metric.Unit(), valueStr, attrStr)
}

// WriteMetricDetailed writes full details of a metric in markdown
func (w *MetricWriter) WriteMetricDetailed(sb *strings.Builder, metric pmetric.Metric, serviceName string, resourceAttrs pcommon.Map) {
	fmt.Fprintf(sb, "## Metric: %s\n\n", metric.Name())
	fmt.Fprintf(sb, "**Type:** %s\n\n", metric.Type().String())
	fmt.Fprintf(sb, "**Unit:** %s\n\n", metric.Unit())
	fmt.Fprintf(sb, "**Service:** %s\n\n", serviceName)
	fmt.Fprintf(sb, "**Description:** %s\n\n", metric.Description())

	switch metric.Type() {
	case pmetric.MetricTypeSum:
		w.writeSumDetailedDataPoints(sb, metric.Sum())
	case pmetric.MetricTypeGauge:
		w.writeGaugeDetailedDataPoints(sb, metric.Gauge())
	case pmetric.MetricTypeHistogram:
		w.writeHistogramDetailedDataPoints(sb, metric.Histogram())
	case pmetric.MetricTypeSummary:
		w.writeSummaryDetailedDataPoints(sb, metric.Summary())
	}

	if resourceAttrs.Len() > 0 {
		sb.WriteString("### Resource Attributes\n\n")
		sb.WriteString("| Key | Value |\n")
		sb.WriteString("|-----|-------|\n")
		resourceAttrs.Range(func(k string, v pcommon.Value) bool {
			fmt.Fprintf(sb, "| %s | %s |\n", k, v.AsString())
			return true
		})
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")
}

func (*MetricWriter) writeSumDetailedDataPoints(sb *strings.Builder, sum pmetric.Sum) {
	sb.WriteString("### Data Points\n\n")
	fmt.Fprintf(sb, "**Aggregation:** %s\n\n", sum.AggregationTemporality().String())
	fmt.Fprintf(sb, "**Is Monotonic:** %t\n\n", sum.IsMonotonic())

	sb.WriteString("| Timestamp | Value | Attributes |\n")
	sb.WriteString("|-----------|-------|------------|\n")

	for i := 0; i < sum.DataPoints().Len(); i++ {
		dp := sum.DataPoints().At(i)
		timestamp := time.Unix(0, int64(dp.Timestamp()))
		attrs := formatAttributes(dp.Attributes())
		if attrs == "" {
			attrs = "-"
		}
		fmt.Fprintf(sb, "| %s | %.2f | %s |\n",
			timestamp.Format("15:04:05.000"), dp.DoubleValue(), attrs)
	}
	sb.WriteString("\n")
}

func (*MetricWriter) writeGaugeDetailedDataPoints(sb *strings.Builder, gauge pmetric.Gauge) {
	sb.WriteString("### Data Points\n\n")
	sb.WriteString("| Timestamp | Value | Attributes |\n")
	sb.WriteString("|-----------|-------|------------|\n")

	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)
		timestamp := time.Unix(0, int64(dp.Timestamp()))
		attrs := formatAttributes(dp.Attributes())
		if attrs == "" {
			attrs = "-"
		}
		fmt.Fprintf(sb, "| %s | %.2f | %s |\n",
			timestamp.Format("15:04:05.000"), dp.DoubleValue(), attrs)
	}
	sb.WriteString("\n")
}

func (*MetricWriter) writeHistogramDetailedDataPoints(sb *strings.Builder, hist pmetric.Histogram) {
	sb.WriteString("### Data Points\n\n")
	fmt.Fprintf(sb, "**Aggregation:** %s\n\n", hist.AggregationTemporality().String())

	for i := 0; i < hist.DataPoints().Len(); i++ {
		dp := hist.DataPoints().At(i)
		timestamp := time.Unix(0, int64(dp.Timestamp()))

		fmt.Fprintf(sb, "#### Data Point %d (%s)\n\n", i+1, timestamp.Format("15:04:05.000"))
		fmt.Fprintf(sb, "**Count:** %d\n\n", dp.Count())
		fmt.Fprintf(sb, "**Sum:** %.2f\n\n", dp.Sum())

		if dp.HasMin() {
			fmt.Fprintf(sb, "**Min:** %.2f\n\n", dp.Min())
		}
		if dp.HasMax() {
			fmt.Fprintf(sb, "**Max:** %.2f\n\n", dp.Max())
		}

		if dp.BucketCounts().Len() > 0 {
			sb.WriteString("**Buckets:**\n\n")
			sb.WriteString("| Upper Bound | Count |\n")
			sb.WriteString("|-------------|-------|\n")
			for j := 0; j < dp.ExplicitBounds().Len(); j++ {
				fmt.Fprintf(sb, "| %.2f | %d |\n", dp.ExplicitBounds().At(j), dp.BucketCounts().At(j))
			}
			if dp.BucketCounts().Len() > dp.ExplicitBounds().Len() {
				fmt.Fprintf(sb, "| +Inf | %d |\n", dp.BucketCounts().At(dp.BucketCounts().Len()-1))
			}
			sb.WriteString("\n")
		}

		attrs := formatAttributes(dp.Attributes())
		if attrs != "" {
			fmt.Fprintf(sb, "**Attributes:** %s\n\n", attrs)
		}
	}
}

func (*MetricWriter) writeSummaryDetailedDataPoints(sb *strings.Builder, summ pmetric.Summary) {
	sb.WriteString("### Data Points\n\n")

	for i := 0; i < summ.DataPoints().Len(); i++ {
		dp := summ.DataPoints().At(i)
		timestamp := time.Unix(0, int64(dp.Timestamp()))

		fmt.Fprintf(sb, "#### Data Point %d (%s)\n\n", i+1, timestamp.Format("15:04:05.000"))
		fmt.Fprintf(sb, "**Count:** %d\n\n", dp.Count())
		fmt.Fprintf(sb, "**Sum:** %.2f\n\n", dp.Sum())

		if dp.QuantileValues().Len() > 0 {
			sb.WriteString("**Quantiles:**\n\n")
			sb.WriteString("| Quantile | Value |\n")
			sb.WriteString("|----------|-------|\n")
			for j := 0; j < dp.QuantileValues().Len(); j++ {
				qv := dp.QuantileValues().At(j)
				fmt.Fprintf(sb, "| %.2f | %.2f |\n", qv.Quantile(), qv.Value())
			}
			sb.WriteString("\n")
		}

		attrs := formatAttributes(dp.Attributes())
		if attrs != "" {
			fmt.Fprintf(sb, "**Attributes:** %s\n\n", attrs)
		}
	}
}
