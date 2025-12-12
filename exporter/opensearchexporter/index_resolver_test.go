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

func TestIndexResolver_ResolveLogRecordIndex_WithServiceName(t *testing.T) {
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
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "otel-logs-myservice-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_MissingServiceName(t *testing.T) {
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
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "otel-logs-default-service-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_NoTimeFormat(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:         "otel-logs-%{service.name}",
		LogsIndexFallback: "default-service",
		Dataset:           "default",
		Namespace:         "namespace",
	}

	ld := createTestLogData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "otel-logs-myservice"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_EmptyLogsIndex(t *testing.T) {
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
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "ss4o_logs-default-namespace-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_WithServiceName(t *testing.T) {
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
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "otel-traces-myservice-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_MissingServiceName(t *testing.T) {
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
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "otel-traces-default-service-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_NoTimeFormat(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:         "otel-traces-%{service.name}",
		TracesIndexFallback: "default-service",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	td := createTestTraceData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "otel-traces-myservice"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_EmptyTracesIndex(t *testing.T) {
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
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "ss4o_traces-default-namespace-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_WithCustomAttribute(t *testing.T) {
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
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "otel-traces-myapp-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_UnknownPlaceholder(t *testing.T) {
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
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "otel-traces-unknown-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_MultipleResources(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "%{service.name}-logs",
		LogsIndexFallback:   "default",
		LogsIndexTimeFormat: "",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogDataMultipleResources("app1", "app2")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)

	resource1 := ld.ResourceLogs().At(0).Resource()
	scope1 := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord1 := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index1 := resolver.ResolveLogRecordIndex(cfg, resource1, scope1, logRecord1, ts)

	resource2 := ld.ResourceLogs().At(1).Resource()
	scope2 := ld.ResourceLogs().At(1).ScopeLogs().At(0).Scope()
	logRecord2 := ld.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0)
	index2 := resolver.ResolveLogRecordIndex(cfg, resource2, scope2, logRecord2, ts)

	expected1 := "app1-logs"
	expected2 := "app2-logs"

	if index1 != expected1 {
		t.Errorf("expected %q, got %q", expected1, index1)
	}
	if index2 != expected2 {
		t.Errorf("expected %q, got %q", expected2, index2)
	}
	if index1 == index2 {
		t.Errorf("indices should be different: %q == %q", index1, index2)
	}
}

func TestIndexResolver_ResolveSpanIndex_MultipleResources(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "%{service.name}-traces",
		TracesIndexFallback:   "default",
		TracesIndexTimeFormat: "",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceDataMultipleResources("svc1", "svc2")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)

	resource1 := td.ResourceSpans().At(0).Resource()
	scope1 := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span1 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index1 := resolver.ResolveSpanIndex(cfg, resource1, scope1, span1, ts)

	resource2 := td.ResourceSpans().At(1).Resource()
	scope2 := td.ResourceSpans().At(1).ScopeSpans().At(0).Scope()
	span2 := td.ResourceSpans().At(1).ScopeSpans().At(0).Spans().At(0)
	index2 := resolver.ResolveSpanIndex(cfg, resource2, scope2, span2, ts)

	expected1 := "svc1-traces"
	expected2 := "svc2-traces"

	if index1 != expected1 {
		t.Errorf("expected %q, got %q", expected1, index1)
	}
	if index2 != expected2 {
		t.Errorf("expected %q, got %q", expected2, index2)
	}
	if index1 == index2 {
		t.Errorf("indices should be different: %q == %q", index1, index2)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_MultiplePlaceholders(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "%{service.name}-%{env}-logs",
		LogsIndexFallback:   "default",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogDataWithMultipleAttributes("myservice", "prod")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "myservice-prod-logs-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_MultiplePlaceholders(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "%{service.name}-%{env}-traces",
		TracesIndexFallback:   "default",
		TracesIndexTimeFormat: "yyyy.MM.dd",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceDataWithMultipleAttributes("myservice", "prod")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "myservice-prod-traces-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_MultiplePlaceholdersWithFallback(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "%{service.name}-%{missing}-traces",
		TracesIndexFallback:   "fallback",
		TracesIndexTimeFormat: "yyyy.MM.dd",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "myservice-fallback-traces-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_MultiplePlaceholdersWithFallback(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "%{service.name}-%{missing}-logs",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "myservice-fallback-logs-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_WithCustomAttribute(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{custom.label}",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogDataWithCustomAttribute("myservice", "custom.label", "myapp")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "otel-logs-myapp-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_UnknownPlaceholder(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{nonexistent}",
		LogsIndexFallback:   "",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "otel-logs-unknown-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveLogRecordIndex_AttributePrecedence(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		LogsIndex:           "%{test.key}",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogDataWithPrecedenceAttributes()
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := ld.ResourceLogs().At(0).Resource()
	scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	index := resolver.ResolveLogRecordIndex(cfg, resource, scope, logRecord, ts)
	expected := "log_value"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestIndexResolver_ResolveSpanIndex_AttributePrecedence(t *testing.T) {
	resolver := newIndexResolver()
	cfg := &Config{
		TracesIndex:           "%{test.key}",
		TracesIndexFallback:   "fallback",
		TracesIndexTimeFormat: "",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceDataWithPrecedenceAttributes()
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	resource := td.ResourceSpans().At(0).Resource()
	scope := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	index := resolver.ResolveSpanIndex(cfg, resource, scope, span, ts)
	expected := "span_value"
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

func createTestLogDataMultipleResources(serviceName1, serviceName2 string) plog.Logs {
	ld := plog.NewLogs()
	rl1 := ld.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("service.name", serviceName1)
	sl1 := rl1.ScopeLogs().AppendEmpty()
	logRecord1 := sl1.LogRecords().AppendEmpty()
	logRecord1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	rl2 := ld.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("service.name", serviceName2)
	sl2 := rl2.ScopeLogs().AppendEmpty()
	logRecord2 := sl2.LogRecords().AppendEmpty()
	logRecord2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	return ld
}

func createTestLogDataWithMultipleAttributes(serviceName, env string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	if serviceName != "" {
		rl.Resource().Attributes().PutStr("service.name", serviceName)
	}
	if env != "" {
		rl.Resource().Attributes().PutStr("env", env)
	}
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return ld
}

func createTestTraceDataWithMultipleAttributes(serviceName, env string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	if serviceName != "" {
		rs.Resource().Attributes().PutStr("service.name", serviceName)
	}
	if env != "" {
		rs.Resource().Attributes().PutStr("env", env)
	}
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return td
}

func createTestTraceDataMultipleResources(serviceName1, serviceName2 string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs1 := td.ResourceSpans().AppendEmpty()
	rs1.Resource().Attributes().PutStr("service.name", serviceName1)
	ss1 := rs1.ScopeSpans().AppendEmpty()
	span1 := ss1.Spans().AppendEmpty()
	span1.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	rs2 := td.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutStr("service.name", serviceName2)
	ss2 := rs2.ScopeSpans().AppendEmpty()
	span2 := ss2.Spans().AppendEmpty()
	span2.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	return td
}

func createTestLogDataWithPrecedenceAttributes() plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("test.key", "resource_value")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().Attributes().PutStr("test.key", "scope_value")
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Attributes().PutStr("test.key", "log_value")
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return ld
}

func createTestTraceDataWithPrecedenceAttributes() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("test.key", "resource_value")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().Attributes().PutStr("test.key", "scope_value")
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("test.key", "span_value")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return td
}
