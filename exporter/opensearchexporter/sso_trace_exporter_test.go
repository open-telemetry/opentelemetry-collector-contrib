// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"testing"
	"time"
)

func TestTraceExporter_ResolveIndexName_WithServiceName(t *testing.T) {
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

func TestTraceExporter_ResolveIndexName_MissingServiceName(t *testing.T) {
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

func TestTraceExporter_ResolveIndexName_NoFallback(t *testing.T) {
	resolver := newIndexResolver()
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

func TestTraceExporter_ResolveIndexName_WithCustomAttribute(t *testing.T) {
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

func TestTraceExporter_ResolveIndexName_EmptyTimeFormat(t *testing.T) {
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
