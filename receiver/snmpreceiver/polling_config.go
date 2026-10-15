// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import "slices"

// effectivePollingConfig isolates polling normalization from shared configuration.
// Copy containers and pointed-to settings that the scraper can change; OID
// association slices and enum values are read-only and can retain references.
func (cfg *Config) effectivePollingConfig() *Config {
	selected := *cfg
	selected.Traps = nil
	if selected.Metrics != nil {
		copied := make(map[string]*MetricConfig, len(selected.Metrics))
		for name, metric := range selected.Metrics {
			if metric == nil {
				copied[name] = nil
				continue
			}
			metricCopy := *metric
			metricCopy.ScalarOIDs = slices.Clone(metric.ScalarOIDs)
			metricCopy.ColumnOIDs = slices.Clone(metric.ColumnOIDs)
			if metric.Gauge != nil {
				gaugeCopy := *metric.Gauge
				metricCopy.Gauge = &gaugeCopy
			}
			if metric.Sum != nil {
				sumCopy := *metric.Sum
				metricCopy.Sum = &sumCopy
			}
			copied[name] = &metricCopy
		}
		selected.Metrics = copied
	}
	if selected.Attributes != nil {
		copied := make(map[string]*AttributeConfig, len(selected.Attributes))
		for name, attribute := range selected.Attributes {
			if attribute == nil {
				copied[name] = nil
				continue
			}
			attributeCopy := *attribute
			copied[name] = &attributeCopy
		}
		selected.Attributes = copied
	}
	if selected.ResourceAttributes != nil {
		copied := make(map[string]*ResourceAttributeConfig, len(selected.ResourceAttributes))
		for name, attribute := range selected.ResourceAttributes {
			if attribute == nil {
				copied[name] = nil
				continue
			}
			attributeCopy := *attribute
			copied[name] = &attributeCopy
		}
		selected.ResourceAttributes = copied
	}
	return &selected
}
