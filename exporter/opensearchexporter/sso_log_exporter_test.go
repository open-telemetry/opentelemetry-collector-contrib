// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"testing"
	"time"
)

func TestResolveLogIndexName_WithServiceName(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}",
		LogsIndexFallback:   "default-service",
		LogsIndexTimeFormat: "yyyy.MM.dd",
	}
	attrs := map[string]string{"service.name": "myservice"}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "otel-logs-myservice-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_MissingServiceName(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}",
		LogsIndexFallback:   "default-service",
		LogsIndexTimeFormat: "yyyy.MM.dd",
	}
	attrs := map[string]string{}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "otel-logs-default-service-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_NoFallback(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}",
		LogsIndexFallback:   "",
		LogsIndexTimeFormat: "yyyy.MM.dd",
	}
	attrs := map[string]string{}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "otel-logs-unknown-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_EmptyConfig(t *testing.T) {
	cfg := &Config{}
	attrs := map[string]string{}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := ""
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_EmptyLogsIndex(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "",
		LogsIndexFallback:   "",
		LogsIndexTimeFormat: "yyyy.MM.dd",
	}
	attrs := map[string]string{}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_UnknownPlaceholder(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "otel-logs-%{doesnotexist}",
		LogsIndexFallback:   "",
		LogsIndexTimeFormat: "yyyy.MM.dd",
	}
	attrs := map[string]string{"service.name": "myservice"}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "otel-logs-unknown-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_EmptyTimeFormat(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}",
		LogsIndexFallback:   "default-service",
		LogsIndexTimeFormat: "",
	}
	attrs := map[string]string{"service.name": "myservice"}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "otel-logs-myservice"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_TwoPlaceholders(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}-%{custom.label}",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "yyyy.MM.dd",
	}
	attrs := map[string]string{"service.name": "svc", "custom.label": "foo"}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "otel-logs-svc-foo-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_TwoPlaceholders_OneMissing(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "otel-logs-%{service.name}-%{custom.label}",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "yyyy.MM.dd",
	}
	attrs := map[string]string{"service.name": "svc"}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "otel-logs-svc-fallback-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}

func TestResolveLogIndexName_AttributeWithSpecialChars(t *testing.T) {
	cfg := &Config{
		LogsIndex:           "%{attribute.something}",
		LogsIndexFallback:   "fallback",
		LogsIndexTimeFormat: "yyyy.MM.dd",
	}
	attrs := map[string]string{"attribute.something": "specialValue"}
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	index := resolveLogIndexName(cfg, attrs, ts)
	expected := "specialValue-2025.06.07"
	if index != expected {
		t.Errorf("expected %q, got %q", expected, index)
	}
}
