// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestIndexResolver_ResolveLogIndex_WithServiceName(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}",
		LogsIndexFallback:   "default-service",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "otel-logs-myservice-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogIndex_MissingServiceName(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}",
		LogsIndexFallback:   "default-service",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "otel-logs-default-service-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogIndex_NoTimeFormat(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:         "otel-logs-%{service.name}",
		LogsIndexFallback: "default-service",
		Dataset:           "default",
		Namespace:         "namespace",
	}

	ld := createTestLogData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "otel-logs-myservice"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogIndex_EmptyLogsIndex(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "",
		LogsIndexFallback:   "",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "ss4o_logs-default-namespace-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveTraceIndex_WithServiceName(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "otel-traces-%{service.name}",
		TracesIndexFallback:   "default-service",
		TracesIndexTimeFormat: "yyyy.MM.dd",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveTraceIndex(cfg, td, ts)
	expected := "otel-traces-myservice-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveTraceIndex_MissingServiceName(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "otel-traces-%{service.name}",
		TracesIndexFallback:   "default-service",
		TracesIndexTimeFormat: "yyyy.MM.dd",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceData("")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveTraceIndex(cfg, td, ts)
	expected := "otel-traces-default-service-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveTraceIndex_NoTimeFormat(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:         "otel-traces-%{service.name}",
		TracesIndexFallback: "default-service",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	td := createTestTraceData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveTraceIndex(cfg, td, ts)
	expected := "otel-traces-myservice"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveTraceIndex_EmptyTracesIndex(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "",
		TracesIndexFallback:   "",
		TracesIndexTimeFormat: "yyyy.MM.dd",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveTraceIndex(cfg, td, ts)
	expected := "ss4o_traces-default-namespace-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveTraceIndex_WithCustomAttribute(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "otel-traces-%{custom.label}",
		TracesIndexFallback:   "fallback",
		TracesIndexTimeFormat: "yyyy.MM.dd",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceDataWithCustomAttribute("myservice", "custom.label", "myapp")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveTraceIndex(cfg, td, ts)
	expected := "otel-traces-myapp-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveTraceIndex_UnknownPlaceholder(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "otel-traces-%{nonexistent}",
		TracesIndexFallback:   "",
		TracesIndexTimeFormat: "yyyy.MM.dd",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveTraceIndex(cfg, td, ts)
	expected := "otel-traces-unknown-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestConvertGoTimeFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"yyyy.MM.dd", "2006.01.02"},
		{"yy-MM-dd", "06-01-02"},
		{"yyyy_MM_dd+HH", "2006_01_02+15"},
		{"yyMMdd", "060102"},
		{"yyyy.MM.dd-HH.mm.ss", "2006.01.02-15.04.05"},
	}

	for _, test := range tests {
		result := convertGoTimeFormat(test.input)
		if result != test.expected {
			t.Errorf("convertGoTimeFormat(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// Helper functions to create test data

func createTestLogData(serviceName string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	if serviceName != "" {
		rl.Resource().Attributes().PutStr("service.name", serviceName)
	}
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return ld
}

func createTestTraceData(serviceName string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	if serviceName != "" {
		rs.Resource().Attributes().PutStr("service.name", serviceName)
	}
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return td
}

func createTestLogDataWithCustomAttribute(serviceName, attrKey, attrValue string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	if serviceName != "" {
		rl.Resource().Attributes().PutStr("service.name", serviceName)
	}
	rl.Resource().Attributes().PutStr(attrKey, attrValue)
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return ld
}

func createTestTraceDataWithCustomAttribute(serviceName, attrKey, attrValue string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	if serviceName != "" {
		rs.Resource().Attributes().PutStr("service.name", serviceName)
	}
	rs.Resource().Attributes().PutStr(attrKey, attrValue)
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return td
}
