// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"context"
	"errors"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
)

// MetadataCache is an adapter to prometheus' scrape.Target  and provide only the functionality which is needed
type MetadataCache interface {
	Metadata(metricName string) (scrape.MetricMetadata, bool)
	SharedLabels() labels.Labels
}

type ScrapeManager interface {
	TargetsAll() map[string][]*scrape.Target
}

type dataPoint struct {
	value    float64
	boundary float64
}

// internalMetricMetadata allows looking up metadata for internal scrape metrics
var internalMetricMetadata = map[string]*scrape.MetricMetadata{
	scrapeUpMetricName: {
		Metric: scrapeUpMetricName,
		Type:   textparse.MetricTypeGauge,
		Help:   "The scraping was successful",
	},
	"scrape_duration_seconds": {
		Metric: "scrape_duration_seconds",
		Unit:   "seconds",
		Type:   textparse.MetricTypeGauge,
		Help:   "Duration of the scrape",
	},
	"scrape_samples_scraped": {
		Metric: "scrape_samples_scraped",
		Type:   textparse.MetricTypeGauge,
		Help:   "The number of samples the target exposed",
	},
	"scrape_series_added": {
		Metric: "scrape_series_added",
		Type:   textparse.MetricTypeGauge,
		Help:   "The approximate number of new series in this scrape",
	},
	"scrape_samples_post_metric_relabeling": {
		Metric: "scrape_samples_post_metric_relabeling",
		Type:   textparse.MetricTypeGauge,
		Help:   "The number of samples remaining after metric relabeling was applied",
	},
}

func metadataForMetric(metricName string, mc MetadataCache) (*scrape.MetricMetadata, string) {
	if metadata, ok := internalMetricMetadata[metricName]; ok {
		return metadata, metricName
	}
	if metadata, ok := mc.Metadata(metricName); ok {
		return &metadata, metricName
	}
	// If we didn't find metadata with the original name,
	// try with suffixes trimmed, in-case it is a "merged" metric type.
	normalizedName := normalizeMetricName(metricName)
	if metadata, ok := mc.Metadata(normalizedName); ok {
		if metadata.Type == textparse.MetricTypeCounter {
			return &metadata, metricName
		}
		return &metadata, normalizedName
	}
	// Otherwise, the metric is unknown
	return &scrape.MetricMetadata{
		Metric: metricName,
		Type:   textparse.MetricTypeUnknown,
	}, metricName
}

func getMetadataCache(ctx context.Context) (MetadataCache, error) {
	target, ok := scrape.TargetFromContext(ctx)
	if !ok {
		return nil, errors.New("unable to find target in context")
	}
	metaStore, ok := scrape.MetricMetadataStoreFromContext(ctx)
	if !ok {
		return nil, errors.New("unable to find MetricMetadataStore in context")
	}

	return &mCache{
		target:   target,
		metadata: metaStore,
	}, nil
}

// adapter to get metadata from scrape.Target
type mCache struct {
	target   *scrape.Target
	metadata scrape.MetricMetadataStore
}

func (m *mCache) Metadata(metricName string) (scrape.MetricMetadata, bool) {
	return m.metadata.GetMetadata(metricName)
}

func (m *mCache) SharedLabels() labels.Labels {
	return m.target.DiscoveredLabels()
}
