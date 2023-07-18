// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exceptionsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"

import (
	"context"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

type logsConnector struct {
	config Config

	// Additional dimensions to add to logs.
	dimensions []dimension

	logsConsumer consumer.Logs
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
}

func newLogsConnector(logger *zap.Logger, config component.Config) (*logsConnector, error) {
	cfg := config.(*Config)

	return &logsConnector{
		logger:     logger,
		config:     *cfg,
		dimensions: newDimensions(cfg.Dimensions),
	}, nil
}

// Capabilities implements the consumer interface.
func (c *logsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate logs.
func (c *logsConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	ld := plog.NewLogs()
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		resourceAttr := rspans.Resource().Attributes()
		serviceAttr, ok := resourceAttr.Get(conventions.AttributeServiceName)
		if !ok {
			continue
		}
		serviceName := serviceAttr.Str()
		ilsSlice := rspans.ScopeSpans()
		for j := 0; j < ilsSlice.Len(); j++ {
			sl := c.newScopeLogs(ld)
			ils := ilsSlice.At(j)
			ils.Scope().CopyTo(sl.Scope())
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					if event.Name() == eventNameExc {
						c.attrToLogRecord(sl, serviceName, span, event)
					}
				}
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

func (c *logsConnector) newScopeLogs(ld plog.Logs) plog.ScopeLogs {
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	return sl
}

func (c *logsConnector) attrToLogRecord(sl plog.ScopeLogs, serviceName string, span ptrace.Span, event ptrace.SpanEvent) plog.LogRecord {
	logRecord := sl.LogRecords().AppendEmpty()

	logRecord.SetTimestamp(event.Timestamp())
	logRecord.SetSeverityNumber(plog.SeverityNumberError)
	logRecord.SetSeverityText("ERROR")
	eventAttrs := event.Attributes()
	spanAttrs := span.Attributes()

	// Add common attributes to the log record.
	logRecord.Attributes().PutStr(spanKindKey, traceutil.SpanKindStr(span.Kind()))
	logRecord.Attributes().PutStr(statusCodeKey, traceutil.StatusCodeStr(span.Status().Code()))
	logRecord.Attributes().PutStr(serviceNameKey, serviceName)

	// Add configured dimension attributes to the log record.
	for _, d := range c.dimensions {
		if v, ok := getDimensionValue(d, spanAttrs, eventAttrs); ok {
			logRecord.Attributes().PutStr(d.name, v.Str())
		}
	}

	// Add stacktrace to the log record.
	logRecord.Attributes().PutStr(exceptionStacktraceKey, getValue(eventAttrs, exceptionStacktraceKey))

	// Add HTTP context to the log record.
	for k, v := range extractHTTP(spanAttrs) {
		logRecord.Attributes().PutStr(k, v)
	}
	return logRecord
}

// extractHTTP extracts the HTTP context from span attributes.
func extractHTTP(attr pcommon.Map) map[string]string {
	http := make(map[string]string)
	attr.Range(func(k string, v pcommon.Value) bool {
		if strings.HasPrefix(k, "http.") {
			http[k] = v.Str()
		}
		return true
	})
	return http
}

// getValue returns the value of the attribute with the given key.
func getValue(attr pcommon.Map, key string) string {
	if attrVal, ok := attr.Get(key); ok {
		return attrVal.Str()
	}
	return ""
}
