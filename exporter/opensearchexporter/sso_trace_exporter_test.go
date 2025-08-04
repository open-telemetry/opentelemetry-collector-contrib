// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Helper functions to create test trace data

func TestTraceExporter_ResolveIndexName_WithServiceName(t *testing.T) {
	resolver := NewIndexResolver()
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

func TestTraceExporter_ResolveIndexName_MissingServiceName(t *testing.T) {
	resolver := NewIndexResolver()
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

func TestTraceExporter_ResolveIndexName_NoFallback(t *testing.T) {
	resolver := NewIndexResolver()
	cfg := &Config{
		TracesIndex:           "otel-traces-%{service.name}",
		TracesIndexFallback:   "",
		TracesIndexTimeFormat: "yyyy.MM.dd",
		Dataset:               "default",
		Namespace:             "namespace",
	}

	td := createTestTraceData("")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolver.ResolveTraceIndex(cfg, td, ts)
	expected := "otel-traces-unknown-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestTraceExporter_ResolveIndexName_EmptyTracesIndex(t *testing.T) {
	resolver := NewIndexResolver()
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

func TestTraceExporter_ResolveIndexName_WithCustomAttribute(t *testing.T) {
	resolver := NewIndexResolver()
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

func TestTraceExporter_ResolveIndexName_EmptyTimeFormat(t *testing.T) {
	resolver := NewIndexResolver()
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

// Helper functions to create test trace data

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