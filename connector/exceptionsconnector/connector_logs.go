// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exceptionsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

type logsConnector struct {
	config Config

	// Additional dimensions to add to logs.
	dimensions []pdatautil.Dimension

	logsConsumer consumer.Logs
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
}

func newLogsConnector(logger *zap.Logger, config component.Config) *logsConnector {
	cfg := config.(*Config)

	return &logsConnector{
		logger:     logger,
		config:     *cfg,
		dimensions: newDimensions(cfg.Dimensions),
	}
}

// Capabilities implements the consumer interface.
func (*logsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate logs.
func (c *logsConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	ld := plog.NewLogs()
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		resourceAttr := rspans.Resource().Attributes()
		serviceAttr, ok := resourceAttr.Get(string(conventions.ServiceNameKey))
		if !ok {
			continue
		}
		serviceName := serviceAttr.Str()
		ilsSlice := rspans.ScopeSpans()
		for j := 0; j < ilsSlice.Len(); j++ {
			sl := c.newScopeLogs(ld, rspans.Resource())
			ils := ilsSlice.At(j)
			ils.Scope().CopyTo(sl.Scope())
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					if event.Name() == eventNameExc {
						c.attrToLogRecord(sl, serviceName, span, event, resourceAttr)
					}
				}
			}
		}
	}
	return c.exportLogs(ctx, ld)
}

// ConsumeLogs implements the consumer.Logs interface for the logs-to-logs edge: forwards only
// records with event.name == "exception" unchanged, dropping everything else.
func (c *logsConnector) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	ld := plog.NewLogs()
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rlogs := logs.ResourceLogs().At(i)
		var outRL plog.ResourceLogs
		haveRL := false
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slIn := rlogs.ScopeLogs().At(j)
			records := slIn.LogRecords()
			var outSL plog.ScopeLogs
			haveSL := false
			for k := 0; k < records.Len(); k++ {
				lr := records.At(k)
				if lr.EventName() != eventNameExc {
					continue
				}
				if !haveRL {
					outRL = ld.ResourceLogs().AppendEmpty()
					rlogs.Resource().CopyTo(outRL.Resource())
					haveRL = true
				}
				if !haveSL {
					outSL = outRL.ScopeLogs().AppendEmpty()
					slIn.Scope().CopyTo(outSL.Scope())
					haveSL = true
				}
				lr.CopyTo(outSL.LogRecords().AppendEmpty())
			}
		}
	}
	return c.exportLogs(ctx, ld)
}

func (c *logsConnector) exportLogs(ctx context.Context, ld plog.Logs) error {
	if err := c.logsConsumer.ConsumeLogs(ctx, ld); err != nil {
		c.logger.Error("failed to convert exceptions to logs", zap.Error(err))
		return err
	}
	return nil
}

// newScopeLogs starts a new ResourceLogs/ScopeLogs pair, copying the full source resource so
// converted LogEvents carry the same resource context as natively-emitted ones.
func (*logsConnector) newScopeLogs(ld plog.Logs, resource pcommon.Resource) plog.ScopeLogs {
	rl := ld.ResourceLogs().AppendEmpty()
	resource.CopyTo(rl.Resource())
	sl := rl.ScopeLogs().AppendEmpty()
	return sl
}

func (c *logsConnector) attrToLogRecord(sl plog.ScopeLogs, serviceName string, span ptrace.Span, event ptrace.SpanEvent, resourceAttrs pcommon.Map) plog.LogRecord {
	logRecord := sl.LogRecords().AppendEmpty()

	logRecord.SetTimestamp(event.Timestamp())
	logRecord.SetSeverityNumber(plog.SeverityNumberError)
	logRecord.SetSeverityText("ERROR")
	logRecord.SetEventName(eventNameExc)
	logRecord.SetSpanID(span.SpanID())
	logRecord.SetTraceID(span.TraceID())
	eventAttrs := event.Attributes()
	spanAttrs := span.Attributes()

	// Copy span attributes to the log record.
	spanAttrs.CopyTo(logRecord.Attributes())

	// Add common attributes to the log record.
	logRecord.Attributes().PutStr(spanNameKey, span.Name())
	logRecord.Attributes().PutStr(spanKindKey, traceutil.SpanKindStr(span.Kind()))
	logRecord.Attributes().PutStr(statusCodeKey, traceutil.StatusCodeStr(span.Status().Code()))
	logRecord.Attributes().PutStr(serviceNameKey, serviceName)

	// Add configured dimension attributes to the log record.
	for _, d := range c.dimensions {
		if v, ok := pdatautil.GetDimensionValue(d, spanAttrs, eventAttrs, resourceAttrs); ok {
			logRecord.Attributes().PutStr(d.Name, v.Str())
		}
	}

	// Set unconditionally (not via the configurable dimensions above): the exception semantic
	// conventions require at least one of type/message, and recommend stacktrace.
	for _, key := range []string{exceptionTypeKey, exceptionMessageKey, exceptionStacktraceKey} {
		if v, ok := pdatautil.GetAttributeValue(key, eventAttrs); ok {
			logRecord.Attributes().PutStr(key, v)
		}
	}
	return logRecord
}
