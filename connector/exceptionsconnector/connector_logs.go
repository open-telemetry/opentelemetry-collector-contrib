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

package exceptionsconnector

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

type logsConnector struct {
	logsConsumer consumer.Logs
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
	ld     plog.Logs
}

func newLogsConnector(logger *zap.Logger) (*logsConnector, error) {
	logger.Info("Building logs exceptionsconnector")

	return &logsConnector{
		logger: logger,
		ld:     plog.NewLogs(),
	}, nil
}

// Capabilities implements the consumer interface.
func (c *logsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate logs.
func (c *logsConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	c.logger.Debug("Consume traces")
	sl := c.newScopeLogs()
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
			ils := ilsSlice.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					if event.Name() == "exception" {
						c.attrToLogRecord(sl, serviceName, span.Attributes(), event.Attributes(), event.Timestamp())
					}
				}
			}
		}
	}
	c.exportLogs(ctx)
	return nil
}

func (c *logsConnector) exportLogs(ctx context.Context) error {
	c.logger.Debug("Exporting logs")
	if err := c.logsConsumer.ConsumeLogs(ctx, c.ld); err != nil {
		c.logger.Error("Failed ConsumeLogs", zap.Error(err))
		return err
	}
	return nil
}

func (c *logsConnector) newScopeLogs() plog.ScopeLogs {
	rl := c.ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("exceptionsconnector")
	return sl
}

func (c *logsConnector) attrToLogRecord(sl plog.ScopeLogs, serviceName string, spanAttr, eventAttr pcommon.Map, eventTs pcommon.Timestamp) plog.LogRecord {
	logRecord := sl.LogRecords().AppendEmpty()

	logRecord.SetTimestamp(eventTs)
	logRecord.SetSeverityNumber(plog.SeverityNumberError)
	logRecord.SetSeverityText("ERROR")

	logRecord.Attributes().PutStr("service.name", serviceName)
	logRecord.Attributes().PutStr("exception.stacktrace", getValue(eventAttr, "exception.stacktrace"))
	logRecord.Attributes().PutStr("exception.type", getValue(eventAttr, "exception.type"))
	logRecord.Attributes().PutStr("exception.message", getValue(eventAttr, "exception.message"))

	for k, v := range extractHTTP(spanAttr) {
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
