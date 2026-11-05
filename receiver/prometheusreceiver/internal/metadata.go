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

var (
	internalUpMetadata = scrape.MetricMetadata{
		MetricFamily: scrapeUpMetricName,
		Type:         model.MetricTypeGauge,
		Help:         "The scraping was successful",
	}
	internalScrapeDurationMetadata = scrape.MetricMetadata{
		MetricFamily: "scrape_duration_seconds",
		Unit:         "seconds",
		Type:         model.MetricTypeGauge,
		Help:         "Duration of the scrape",
	}
	internalScrapeSamplesMetadata = scrape.MetricMetadata{
		MetricFamily: "scrape_samples_scraped",
		Type:         model.MetricTypeGauge,
		Help:         "The number of samples the target exposed",
	}
	internalScrapeSeriesAddedMetadata = scrape.MetricMetadata{
		MetricFamily: "scrape_series_added",
		Type:         model.MetricTypeGauge,
		Help:         "The approximate number of new series in this scrape",
	}
	internalScrapeSamplesPostRelabelingMetadata = scrape.MetricMetadata{
		MetricFamily: "scrape_samples_post_metric_relabeling",
		Type:         model.MetricTypeGauge,
		Help:         "The number of samples remaining after metric relabeling was applied",
	}
)

// lookupInternalMetricMetadata looks up metadata for internal scrape metrics.
func lookupInternalMetricMetadata(metricName string) *scrape.MetricMetadata {
	switch metricName {
	case scrapeUpMetricName:
		return &internalUpMetadata
	case "scrape_duration_seconds":
		return &internalScrapeDurationMetadata
	case "scrape_samples_scraped":
		return &internalScrapeSamplesMetadata
	case "scrape_series_added":
		return &internalScrapeSeriesAddedMetadata
	case "scrape_samples_post_metric_relabeling":
		return &internalScrapeSamplesPostRelabelingMetadata
	default:
		return nil
	}
}

func metadataForMetric(metricName string, mc scrape.MetricMetadataStore) (scrape.MetricMetadata, string) {
	if metadata := lookupInternalMetricMetadata(metricName); metadata != nil {
		return *metadata, metricName
	}
	if metadata, ok := mc.GetMetadata(metricName); ok {
		return metadata, metricName
	}
	// If we didn't find metadata with the original name,
	// try with suffixes trimmed, in-case it is a "merged" metric type.
	normalizedName := normalizeMetricName(metricName)
	if normalizedName != metricName {
		if metadata, ok := mc.GetMetadata(normalizedName); ok {
			if metadata.Type == model.MetricTypeCounter {
				// NB (eriksywu): see https://github.com/prometheus/prometheus/issues/14823
				if strings.HasSuffix(metricName, metricSuffixCreated) {
					return metadata, normalizedName + metricSuffixTotal
				}
				// END NB (eriksywu)
				return metadata, metricName
			}
			return metadata, normalizedName
		}
	}
	// Otherwise, the metric is unknown
	return scrape.MetricMetadata{
		MetricFamily: metricName,
		Type:         model.MetricTypeUnknown,
	}, metricName
}

type emptyMetadataStore struct{}

var emptyMetadataStoreInstance = &emptyMetadataStore{}

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
