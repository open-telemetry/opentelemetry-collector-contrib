// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starrocksexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter"

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// debugLogsExporter handles debug mode for logs - logs data instead of sending to destination.
type debugLogsExporter struct {
	logger *zap.Logger
	config *Config
}

// newDebugLogsExporter creates a new debug logs exporter.
func newDebugLogsExporter(logger *zap.Logger, cfg *Config) *debugLogsExporter {
	return &debugLogsExporter{
		logger: logger,
		config: cfg,
	}
}

func (e *debugLogsExporter) start(ctx context.Context, host component.Host) error {
	e.logger.Info("Debug logs exporter started - data will be logged instead of sent to StarRocks")
	return nil
}

func (e *debugLogsExporter) shutdown(_ context.Context) error {
	return nil
}

// pushLogsData logs log data for debugging.
func (e *debugLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	// Basic logging with counts
	logCount := ld.LogRecordCount()
	resourceCount := ld.ResourceLogs().Len()

	e.logger.Debug("Debug: Logs received",
		zap.Int("log_count", logCount),
		zap.Int("resource_logs_count", resourceCount))

	// Log detailed information for each resource log
	for i := 0; i < resourceCount; i++ {
		rl := ld.ResourceLogs().At(i)
		scopeCount := rl.ScopeLogs().Len()
		e.logger.Debug("Debug: Resource log",
			zap.Int("resource_index", i),
			zap.String("resource_schema_url", rl.SchemaUrl()),
			zap.Int("scope_logs_count", scopeCount),
			zap.Int("resource_attributes_count", rl.Resource().Attributes().Len()))

		// Log each scope log
		for j := 0; j < scopeCount; j++ {
			sl := rl.ScopeLogs().At(j)
			recordCount := sl.LogRecords().Len()
			e.logger.Debug("Debug: Scope log",
				zap.Int("resource_index", i),
				zap.Int("scope_index", j),
				zap.String("scope_name", sl.Scope().Name()),
				zap.String("scope_version", sl.Scope().Version()),
				zap.Int("log_records_count", recordCount))

			// Log first few log records as JSON examples
			maxExamples := 3
			for k := 0; k < recordCount && k < maxExamples; k++ {
				record := sl.LogRecords().At(k)
				example := map[string]interface{}{
					"time":             record.Timestamp().AsTime().String(),
					"severity_number":  record.SeverityNumber(),
					"severity_text":    record.SeverityText(),
					"body":             record.Body().AsString(),
					"attributes_count": record.Attributes().Len(),
					"trace_id":         record.TraceID().String(),
					"span_id":          record.SpanID().String(),
					"flags":            record.Flags(),
				}
				if jsonBytes, err := json.Marshal(example); err == nil {
					e.logger.Debug("Debug: Log record example",
						zap.Int("resource_index", i),
						zap.Int("scope_index", j),
						zap.Int("record_index", k),
						zap.String("record", string(jsonBytes)))
				}
			}
		}
	}

	return nil
}

// debugTracesExporter handles debug mode for traces - logs data instead of sending to destination.
type debugTracesExporter struct {
	logger *zap.Logger
	config *Config
}

// newDebugTracesExporter creates a new debug traces exporter.
func newDebugTracesExporter(logger *zap.Logger, cfg *Config) *debugTracesExporter {
	return &debugTracesExporter{
		logger: logger,
		config: cfg,
	}
}

func (e *debugTracesExporter) start(ctx context.Context, host component.Host) error {
	e.logger.Info("Debug traces exporter started - data will be logged instead of sent to StarRocks")
	return nil
}

func (e *debugTracesExporter) shutdown(_ context.Context) error {
	return nil
}

// pushTraceData logs trace data for debugging.
func (e *debugTracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	// Count spans
	spanCount := 0
	rsSpans := td.ResourceSpans()
	resourceCount := rsSpans.Len()

	for i := 0; i < resourceCount; i++ {
		rs := rsSpans.At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			spanCount += ss.Spans().Len()
		}
	}

	e.logger.Debug("Debug: Traces received",
		zap.Int("span_count", spanCount),
		zap.Int("resource_spans_count", resourceCount))

	// Log detailed information for each resource span
	for i := 0; i < resourceCount; i++ {
		rs := rsSpans.At(i)
		scopeCount := rs.ScopeSpans().Len()
		e.logger.Debug("Debug: Resource spans",
			zap.Int("resource_index", i),
			zap.String("resource_schema_url", rs.SchemaUrl()),
			zap.Int("scope_spans_count", scopeCount),
			zap.Int("resource_attributes_count", rs.Resource().Attributes().Len()))

		// Log each scope span
		for j := 0; j < scopeCount; j++ {
			ss := rs.ScopeSpans().At(j)
			spans := ss.Spans()
			spansCount := spans.Len()
			e.logger.Debug("Debug: Scope spans",
				zap.Int("resource_index", i),
				zap.Int("scope_index", j),
				zap.String("scope_name", ss.Scope().Name()),
				zap.String("scope_version", ss.Scope().Version()),
				zap.Int("spans_count", spansCount))

			// Log first few spans as JSON examples
			maxExamples := 2
			for k := 0; k < spansCount && k < maxExamples; k++ {
				span := spans.At(k)
				example := map[string]interface{}{
					"trace_id":         span.TraceID().String(),
					"span_id":          span.SpanID().String(),
					"parent_span_id":   span.ParentSpanID().String(),
					"name":             span.Name(),
					"kind":             span.Kind().String(),
					"start_time":       span.StartTimestamp().AsTime().String(),
					"end_time":         span.EndTimestamp().AsTime().String(),
					"status_code":      span.Status().Code().String(),
					"status_message":   span.Status().Message(),
					"attributes_count": span.Attributes().Len(),
					"events_count":     span.Events().Len(),
					"links_count":      span.Links().Len(),
				}
				if jsonBytes, err := json.Marshal(example); err == nil {
					e.logger.Debug("Debug: Span example",
						zap.Int("resource_index", i),
						zap.Int("scope_index", j),
						zap.Int("span_index", k),
						zap.String("span", string(jsonBytes)))
				}
			}
		}
	}

	return nil
}

// debugMetricsExporter handles debug mode for metrics - logs data instead of sending to destination.
type debugMetricsExporter struct {
	logger *zap.Logger
	config *Config
}

// newDebugMetricsExporter creates a new debug metrics exporter.
func newDebugMetricsExporter(logger *zap.Logger, cfg *Config) *debugMetricsExporter {
	return &debugMetricsExporter{
		logger: logger,
		config: cfg,
	}
}

func (e *debugMetricsExporter) start(ctx context.Context, host component.Host) error {
	e.logger.Info("Debug metrics exporter started - data will be logged instead of sent to StarRocks")
	return nil
}

func (e *debugMetricsExporter) shutdown(_ context.Context) error {
	return nil
}

// pushMetricsData logs metric data for debugging.
func (e *debugMetricsExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	// Count metrics and data points
	metricCount := 0
	dataPointCount := 0
	rsMetrics := md.ResourceMetrics()
	resourceCount := rsMetrics.Len()

	for i := 0; i < resourceCount; i++ {
		rs := rsMetrics.At(i)
		for j := 0; j < rs.ScopeMetrics().Len(); j++ {
			sm := rs.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				metricCount++
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dataPointCount += metric.Gauge().DataPoints().Len()
				case pmetric.MetricTypeSum:
					dataPointCount += metric.Sum().DataPoints().Len()
				case pmetric.MetricTypeSummary:
					dataPointCount += metric.Summary().DataPoints().Len()
				case pmetric.MetricTypeHistogram:
					dataPointCount += metric.Histogram().DataPoints().Len()
				case pmetric.MetricTypeExponentialHistogram:
					dataPointCount += metric.ExponentialHistogram().DataPoints().Len()
				}
			}
		}
	}

	e.logger.Debug("Debug: Metrics received",
		zap.Int("metric_count", metricCount),
		zap.Int("data_point_count", dataPointCount),
		zap.Int("resource_metrics_count", resourceCount))

	// Log detailed information for each resource metric
	for i := 0; i < resourceCount; i++ {
		rs := rsMetrics.At(i)
		scopeCount := rs.ScopeMetrics().Len()
		e.logger.Debug("Debug: Resource metrics",
			zap.Int("resource_index", i),
			zap.String("resource_schema_url", rs.SchemaUrl()),
			zap.Int("scope_metrics_count", scopeCount),
			zap.Int("resource_attributes_count", rs.Resource().Attributes().Len()))

		// Log each scope metric
		for j := 0; j < scopeCount; j++ {
			sm := rs.ScopeMetrics().At(j)
			metrics := sm.Metrics()
			metricsCount := metrics.Len()
			e.logger.Debug("Debug: Scope metrics",
				zap.Int("resource_index", i),
				zap.Int("scope_index", j),
				zap.String("scope_name", sm.Scope().Name()),
				zap.String("scope_version", sm.Scope().Version()),
				zap.Int("metrics_count", metricsCount))

			// Log first few metrics as JSON examples
			maxExamples := 3
			for k := 0; k < metricsCount && k < maxExamples; k++ {
				metric := metrics.At(k)
				example := map[string]interface{}{
					"name":        metric.Name(),
					"description": metric.Description(),
					"unit":        metric.Unit(),
					"type":        metric.Type().String(),
				}
				if jsonBytes, err := json.Marshal(example); err == nil {
					e.logger.Debug("Debug: Metric example",
						zap.Int("resource_index", i),
						zap.Int("scope_index", j),
						zap.Int("metric_index", k),
						zap.String("metric", string(jsonBytes)))
				}
			}
		}
	}

	return nil
}
