// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/scrape"

	fakemetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
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
	store := fakemetadata.NewFakeMetadataStore(map[string]scrape.MetricMetadata{
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
	store := fakemetadata.NewFakeMetadataStore(map[string]scrape.MetricMetadata{
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
	store := fakemetadata.NewFakeMetadataStore(map[string]scrape.MetricMetadata{
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
	store := fakemetadata.NewFakeMetadataStore(map[string]scrape.MetricMetadata{
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
