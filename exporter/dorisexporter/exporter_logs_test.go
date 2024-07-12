// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
)

func TestStartLogs(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.MySQLEndpoint = "127.0.0.1:9030"
	config.Username = "admin"
	config.Password = "admin"
	config.Database = "otel2"
	config.Table.Logs = "logs"
	config.HistoryDays = 3

	exporter, err := newLogsExporter(nil, config)
	ctx := context.Background()
	if err != nil {
		t.Error(err)
		return
	}
	defer exporter.shutdown(ctx)

	err = exporter.start(ctx, nil)
	if err != nil {
		t.Error(err)
		return
	}
}

var logs = []*dLog{
	{
		ServiceName:        "service_name",
		Timestamp:          time.Now().Format(timeFormat),
		TraceID:            "trace_id",
		SpanID:             "span_id",
		SeverityNumber:     9,
		SeverityText:       "severity_text",
		Body:               "body",
		ResourceAttributes: map[string]any{"k1": "v1", "k2": 2},
		LogAttributes:      map[string]any{"k1": "v1", "k2": 2},
		ScopeName:          "scope_name",
		ScopeVersion:       "scope_version",
	},
	{
		ServiceName:        "service_name",
		Timestamp:          time.Now().Format(timeFormat),
		TraceID:            "trace_id",
		SpanID:             "span_id",
		SeverityNumber:     9,
		SeverityText:       "severity_text",
		Body:               "body",
		ResourceAttributes: map[string]any{"k1": "v1", "k2": 2},
		LogAttributes:      map[string]any{"k1": "v1", "k2": 2},
		ScopeName:          "scope_name",
		ScopeVersion:       "scope_version",
	},
}

func TestPushLogDataInternal(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "http://127.0.0.1:8030"
	config.MySQLEndpoint = "127.0.0.1:9030"
	config.Username = "admin"
	config.Password = "admin"
	config.Database = "otel2"
	config.Table.Logs = "logs"
	config.HistoryDays = 3

	exporter, err := newLogsExporter(nil, config)
	ctx := context.Background()
	if err != nil {
		t.Error(err)
		return
	}
	defer exporter.shutdown(ctx)

	err = exporter.start(ctx, nil)
	if err != nil {
		t.Error(err)
		return
	}

	// no timezone
	err = exporter.pushLogDataInternal(ctx, logs)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestPushLogData(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "http://127.0.0.1:8030"
	config.MySQLEndpoint = "127.0.0.1:9030"
	config.Username = "admin"
	config.Password = "admin"
	config.Database = "otel2"
	config.Table.Logs = "logs"
	config.HistoryDays = 3
	config.CreateHistoryDays = 1
	config.TimeZone = "America/New_York"

	err := config.Validate()
	if err != nil {
		t.Error(err)
		return
	}

	exporter, err := newLogsExporter(nil, config)
	ctx := context.Background()
	if err != nil {
		t.Error(err)
		return
	}
	defer exporter.shutdown(ctx)

	err = exporter.start(ctx, nil)
	if err != nil {
		t.Error(err)
		return
	}

	err = exporter.pushLogData(ctx, simpleLogs(10))
	if err != nil {
		t.Error(err)
		return
	}
}

func simpleLogs(count int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("io.opentelemetry.contrib.doris")
	sl.Scope().SetVersion("1.0.0")
	sl.Scope().Attributes().PutStr("lib", "doris")
	timestamp := time.Now()
	for i := 0; i < count; i++ {
		r := sl.LogRecords().AppendEmpty()
		r.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetSeverityNumber(plog.SeverityNumberError2)
		r.SetSeverityText("error")
		r.Body().SetStr("error message")
		r.Attributes().PutStr(semconv.AttributeServiceNamespace, "default")
		r.SetFlags(plog.DefaultLogRecordFlags)
		r.SetTraceID([16]byte{1, 2, 3, byte(i)})
		r.SetSpanID([8]byte{1, 2, 3, byte(i)})
	}
	return logs
}
