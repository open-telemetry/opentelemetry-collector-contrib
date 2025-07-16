// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
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
			return &metadata, metricName
		}
		return &metadata, normalizedName
	}
	// Otherwise, the metric is unknown
	return &scrape.MetricMetadata{
		MetricFamily: metricName,
		Type:         model.MetricTypeUnknown,
	}, metricName
}
