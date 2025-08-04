// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestLogExporter_ResolveIndexName_WithServiceName(t *testing.T) {
	resolver := NewIndexResolver()
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

func TestLogExporter_ResolveIndexName_MissingServiceName(t *testing.T) {
	resolver := NewIndexResolver()
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

func TestLogExporter_ResolveIndexName_NoFallback(t *testing.T) {
	resolver := NewIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}",
		LogsIndexFallback:   "",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "otel-logs-unknown-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestLogExporter_ResolveIndexName_EmptyLogsIndex(t *testing.T) {
	resolver := NewIndexResolver()
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


func TestLogExporter_ResolveIndexName_UnknownPlaceholder(t *testing.T) {
	resolver := NewIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{doesnotexist}",
		LogsIndexFallback:   "",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "otel-logs-unknown-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestLogExporter_ResolveIndexName_EmptyTimeFormat(t *testing.T) {
	resolver := NewIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}",
		LogsIndexFallback:   "default-service",
		LogsIndexTimeFormat: "",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("myservice")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "otel-logs-myservice"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestLogExporter_ResolveIndexName_TwoPlaceholders(t *testing.T) {
	resolver := NewIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}-%{custom.label}",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogDataWithCustomAttribute("svc", "custom.label", "foo")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "otel-logs-svc-foo-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestLogExporter_ResolveIndexName_TwoPlaceholders_OneMissing(t *testing.T) {
	resolver := NewIndexResolver()
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}-%{custom.label}",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogData("svc")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "otel-logs-svc-fallback-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestLogExporter_ResolveIndexName_AttributeWithSpecialChars(t *testing.T) {
	resolver := NewIndexResolver()
	cfg := &Config{
		LogsIndex:           "%{attribute.something}",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "yyyy.MM.dd",
		Dataset:             "default",
		Namespace:           "namespace",
	}

	ld := createTestLogDataWithCustomAttribute("", "attribute.something", "specialValue")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveLogIndex(cfg, ld, ts)
	expected := "specialValue-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

// Helper functions to create test log data

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
