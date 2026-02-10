// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/scrape"
)

type dataPoint struct {
	value    float64
	boundary float64
}

// internalMetricMetadata allows looking up metadata for internal scrape metrics
var internalMetricMetadata = map[string]*scrape.MetricMetadata{
	scrapeUpMetricName: {
		MetricFamily: scrapeUpMetricName,
		Type:         model.MetricTypeGauge,
		Help:         "The scraping was successful",
	},
	"scrape_duration_seconds": {
		MetricFamily: "scrape_duration_seconds",
		Unit:         "seconds",
		Type:         model.MetricTypeGauge,
		Help:         "Duration of the scrape",
	},
	"scrape_samples_scraped": {
		MetricFamily: "scrape_samples_scraped",
		Type:         model.MetricTypeGauge,
		Help:         "The number of samples the target exposed",
	},
	"scrape_series_added": {
		MetricFamily: "scrape_series_added",
		Type:         model.MetricTypeGauge,
		Help:         "The approximate number of new series in this scrape",
	},
	"scrape_samples_post_metric_relabeling": {
		MetricFamily: "scrape_samples_post_metric_relabeling",
		Type:         model.MetricTypeGauge,
		Help:         "The number of samples remaining after metric relabeling was applied",
	},
}

func metadataForMetric(metricName string, mc scrape.MetricMetadataStore) (*scrape.MetricMetadata, string) {
	if metadata, ok := internalMetricMetadata[metricName]; ok {
		return metadata, metricName
	}
	if metadata, ok := mc.GetMetadata(metricName); ok {
		return &metadata, metricName
	}
	// If we didn't find metadata with the original name,
	// try with suffixes trimmed, in-case it is a "merged" metric type.
	normalizedName := normalizeMetricName(metricName)
	if metadata, ok := mc.GetMetadata(normalizedName); ok {
		if metadata.Type == model.MetricTypeCounter {
			// NB (eriksywu): see https://github.com/prometheus/prometheus/issues/14823
			if strings.HasSuffix(metricName, metricSuffixCreated) {
				return &metadata, normalizedName + metricSuffixTotal
			}
			// END NB (eriksywu)
			return &metadata, metricName
		}
		return &metadata, normalizedName
	}
	// Otherwise, the metric is unknown.
	// As a fallback, try to infer the type from the metric name to handle
	// cases where metadata is not available (e.g., issue #34263).
	// Use Prometheus naming conventions to detect counters.
	if isLikelyCounter(metricName) {
		return &scrape.MetricMetadata{
			MetricFamily: normalizedName,
			Type:         model.MetricTypeCounter,
		}, metricName
	}

	return &scrape.MetricMetadata{
		MetricFamily: metricName,
		Type:         model.MetricTypeUnknown,
	}, metricName
}

// isLikelyCounter uses heuristics to determine if a metric name likely represents a counter
// when metadata is not available. This follows Prometheus naming conventions and common patterns.
func isLikelyCounter(metricName string) bool {
	// Explicit counter suffixes (Prometheus conventions)
	// Note: _count is NOT included here because it's ambiguous - it could be part of
	// histogram metrics (foo_count) or just a poorly named metric (some_count)
	counterSuffixes := []string{
		metricSuffixTotal,  // "_total" - most common and reliable counter suffix
		"_created",         // timestamp when counter was created
		"_bytes_total",     // total bytes
		"_packets_total",   // total packets
	}

	for _, suffix := range counterSuffixes {
		if strings.HasSuffix(metricName, suffix) {
			return true
		}
	}

	// Common counter patterns (operations that accumulate over time)
	// These are substrings that indicate cumulative/monotonically increasing values
	counterPatterns := []string{
		"_read",      // bytes/blocks/records read (e.g., cnpg_pg_stat_database_blks_read)
		"_written",   // bytes/blocks/records written
		"_sent",      // messages/bytes sent
		"_received",  // messages/bytes received
		"_requests",  // number of requests
		"_errors",    // number of errors
		"_failures",  // number of failures
		"_success",   // number of successes
		"_hits",      // cache hits, etc.
		"_misses",    // cache misses, etc.
		"_packets",   // network packets
		"_dropped",   // dropped packets/messages
		"_processed", // processed items
		"_completed", // completed operations
		"_failed",    // failed operations
		"_retries",   // retry attempts
	}

	for _, pattern := range counterPatterns {
		if strings.Contains(metricName, pattern) {
			// Avoid false positives: if it ends with "_ratio" or "_percent", it's likely a gauge
			if strings.HasSuffix(metricName, "_ratio") || strings.HasSuffix(metricName, "_percent") {
				continue
			}
			return true
		}
	}

	return false
}

// isCounterCreatedLine determines whether a metric is a _created line for a counter appended by an om-text parser
// these assumptions are made
// 1. the metric name of an OM counter line would always have either _total or _created as a suffix
// 2. the omptextarser stores metadata of every om text line using the counter's normalized name (i.e foo_counter_total => foo_counter, foo_counter_created => foo_counter)
// 3. the promtextparser stores metadata without normalization of metric name
func isCounterCreatedLine(metricName, normalizedMetricName string, mc scrape.MetricMetadataStore) bool {
	if !strings.HasSuffix(metricName, metricSuffixCreated) {
		return false
	}
	md, ok := mc.GetMetadata(normalizedMetricName)
	return ok && md.Type == model.MetricTypeCounter
}

type emptyMetadataStore struct{}

func (emptyMetadataStore) ListMetadata() []scrape.MetricMetadata {
	return nil
}

func (emptyMetadataStore) GetMetadata(string) (scrape.MetricMetadata, bool) {
	return scrape.MetricMetadata{}, false
}

func (emptyMetadataStore) SizeMetadata() int {
	return 0
}

func (emptyMetadataStore) LengthMetadata() int {
	return 0
}
