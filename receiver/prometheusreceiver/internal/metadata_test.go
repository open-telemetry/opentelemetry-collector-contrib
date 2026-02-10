// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"maps"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/scrape"
)

func TestMetadataForMetric_Internal(t *testing.T) {
	// Internal metric should return from internalMetricMetadata, ignoring external store.
	metadata, resolved := metadataForMetric(scrapeUpMetricName, emptyMetadataStore{})
	if resolved != scrapeUpMetricName {
		t.Fatalf("expected resolved name %q, got %q", scrapeUpMetricName, resolved)
	}
	if metadata.Type != model.MetricTypeGauge {
		t.Fatalf("expected type Gauge, got %v", metadata.Type)
	}
	if metadata.MetricFamily != scrapeUpMetricName {
		t.Fatalf("expected family %q, got %q", scrapeUpMetricName, metadata.MetricFamily)
	}
}

func TestMetadataForMetric_ExternalExactHit(t *testing.T) {
	store := newFakeMetadataStore(map[string]scrape.MetricMetadata{
		"http_requests_total": {
			MetricFamily: "http_requests_total",
			Type:         model.MetricTypeCounter,
			Help:         "Total HTTP requests",
		},
	},
	)
	metadata, resolved := metadataForMetric("http_requests_total", store)
	if resolved != "http_requests_total" {
		t.Fatalf("expected resolved name http_requests_total, got %q", resolved)
	}
	if metadata.Type != model.MetricTypeCounter {
		t.Fatalf("expected Counter, got %v", metadata.Type)
	}
}

func TestMetadataForMetric_NormalizedFallback_Gauge(t *testing.T) {
	// Simulate a merged metric like "histogram_count" where store only knows the base name.
	store := newFakeMetadataStore(map[string]scrape.MetricMetadata{
		"histogram": {
			MetricFamily: "histogram",
			Type:         model.MetricTypeGauge,
			Help:         "Histogram base metric",
		},
	})
	metadata, resolved := metadataForMetric("histogram_count", store)
	// For non-counter types, resolved should be the normalized base name.
	if resolved != "histogram" {
		t.Fatalf("expected resolved name histogram, got %q", resolved)
	}
	if metadata.Type != model.MetricTypeGauge {
		t.Fatalf("expected Gauge, got %v", metadata.Type)
	}
	if metadata.MetricFamily != "histogram" {
		t.Fatalf("expected family histogram, got %q", metadata.MetricFamily)
	}
}

func TestMetadataForMetric_NormalizedFallback_CounterKeepsOriginal(t *testing.T) {
	// If normalized metadata type is Counter, resolved should stay the original name.
	store := newFakeMetadataStore(map[string]scrape.MetricMetadata{
		"requests": {
			MetricFamily: "requests",
			Type:         model.MetricTypeCounter,
			Help:         "Requests counter",
		},
	},
	)
	metadata, resolved := metadataForMetric("requests_total", store)
	if resolved != "requests_total" {
		t.Fatalf("expected resolved name requests_total, got %q", resolved)
	}
	if metadata.Type != model.MetricTypeCounter {
		t.Fatalf("expected Counter, got %v", metadata.Type)
	}
	if metadata.MetricFamily != "requests" {
		t.Fatalf("expected family requests, got %q", metadata.MetricFamily)
	}
}

func TestMetadataForMetric_Unknown(t *testing.T) {
	// Neither internal nor external store has the metric.
	store := emptyMetadataStore{}
	const name = "custom_metric_unknown"
	metadata, resolved := metadataForMetric(name, store)
	if resolved != name {
		t.Fatalf("expected resolved name %q, got %q", name, resolved)
	}
	if metadata.Type != model.MetricTypeUnknown {
		t.Fatalf("expected Unknown, got %v", metadata.Type)
	}
	if metadata.MetricFamily != name {
		t.Fatalf("expected family %q, got %q", name, metadata.MetricFamily)
	}
}

func TestMetadataForMetric_PrefersInternalOverExternal(t *testing.T) {
	// Ensure internal metric metadata is used even if external store provides a conflicting entry.
	store := newFakeMetadataStore(map[string]scrape.MetricMetadata{
		scrapeUpMetricName: {
			MetricFamily: "up_external",
			Type:         model.MetricTypeCounter,
			Help:         "External override attempt",
		},
	},
	)
	m, resolved := metadataForMetric(scrapeUpMetricName, store)
	if resolved != scrapeUpMetricName {
		t.Fatalf("expected resolved name %q, got %q", scrapeUpMetricName, resolved)
	}
	// Internal definition should win: Gauge with original family.
	if m.Type != model.MetricTypeGauge {
		t.Fatalf("expected type Gauge from internal map, got %v", m.Type)
	}
	if m.MetricFamily != scrapeUpMetricName {
		t.Fatalf("expected family %q from internal map, got %q", scrapeUpMetricName, m.MetricFamily)
	}
}

// fakeMetadataStore implements scrape.MetricMetadataStore for tests.
// It is safe to use from other packages' tests
type fakeMetadataStore struct {
	data map[string]scrape.MetricMetadata
}

// newFakeMetadataStore creates a FakeMetadataStore initialized with the given metadata.
func newFakeMetadataStore(init map[string]scrape.MetricMetadata) *fakeMetadataStore {
	// copy defensively to avoid external mutation
	cp := make(map[string]scrape.MetricMetadata, len(init))
	maps.Copy(cp, init)
	return &fakeMetadataStore{data: cp}
}

func (f fakeMetadataStore) ListMetadata() []scrape.MetricMetadata {
	out := make([]scrape.MetricMetadata, 0, len(f.data))
	for _, m := range f.data {
		out = append(out, m)
	}
	return out
}

func (f fakeMetadataStore) GetMetadata(name string) (scrape.MetricMetadata, bool) {
	m, ok := f.data[name]
	return m, ok
}

func (f fakeMetadataStore) SizeMetadata() int {
	return len(f.data)
}

func (f fakeMetadataStore) LengthMetadata() int {
	return len(f.data)
}

func TestIsLikelyCounter(t *testing.T) {
	testCases := []struct {
		metricName string
		expected   bool
		reason     string
	}{
		// Explicit counter suffixes
		{"http_requests_total", true, "has _total suffix"},
		{"metric_created", true, "has _created suffix"},
		{"data_bytes_total", true, "has _bytes_total suffix"},
		{"network_packets_total", true, "has _packets_total suffix"},

		// Common counter patterns (issue #34263)
		{"cnpg_pg_stat_database_blks_read", true, "contains _read pattern"},
		{"bytes_written", true, "contains _written pattern"},
		{"messages_sent", true, "contains _sent pattern"},
		{"messages_received", true, "contains _received pattern"},
		{"http_requests", true, "contains _requests pattern"},
		{"http_errors", true, "contains _errors pattern"},
		{"operation_failures", true, "contains _failures pattern"},
		{"cache_hits", true, "contains _hits pattern"},
		{"cache_misses", true, "contains _misses pattern"},
		{"packets_dropped", true, "contains _dropped pattern"},
		{"items_processed", true, "contains _processed pattern"},
		{"tasks_completed", true, "contains _completed pattern"},
		{"operations_failed", true, "contains _failed pattern"},
		{"request_retries", true, "contains _retries pattern"},

		// Should NOT be detected as counters
		{"cpu_usage", false, "gauge - current usage"},
		{"memory_bytes", false, "gauge - current size"},
		{"temperature_celsius", false, "gauge - current temperature"},
		{"queue_length", false, "gauge - current queue length"},
		{"active_connections", false, "gauge - current connections"},
		{"some_count", false, "_count suffix is ambiguous without context"},
		{"success_ratio", false, "ratio is a gauge even if contains success"},
		{"error_percent", false, "percent is a gauge even if contains error"},
		{"cache_hit_ratio", false, "ratio is a gauge even if contains hits"},
	}

	for _, tc := range testCases {
		t.Run(tc.metricName, func(t *testing.T) {
			result := isLikelyCounter(tc.metricName)
			if result != tc.expected {
				t.Errorf("Metric: %s - %s: expected %v, got %v", tc.metricName, tc.reason, tc.expected, result)
			}
		})
	}
}
