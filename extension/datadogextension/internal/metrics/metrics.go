// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/metrics"

import (
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"go.opentelemetry.io/collector/component"
)

// NewGauge creates a new DatadogV2 Gauge metric given a name, a Unix nanoseconds timestamp,
// a value and a slice of tags.
func NewGauge(name string, ts uint64, value float64, tags []string) datadogV2.MetricSeries {
	// Transform UnixNano timestamp into Unix timestamp
	// 1 second = 1e9 ns
	timestamp := int64(ts / 1e9)

	metric := datadogV2.MetricSeries{
		Metric: name,
		Points: []datadogV2.MetricPoint{
			{
				Timestamp: datadog.PtrInt64(timestamp),
				Value:     datadog.PtrFloat64(value),
			},
		},
		Tags: tags,
	}
	metric.SetType(datadogV2.METRICINTAKETYPE_GAUGE)
	return metric
}

// DefaultMetrics creates built-in metrics to report that an extension is running.
func DefaultMetrics(hostname string, timestamp uint64, tags []string) []datadogV2.MetricSeries {
	metrics := []datadogV2.MetricSeries{
		NewGauge("otel.datadog_extension.running", timestamp, 1.0, tags),
	}
	for i := range metrics {
		metrics[i].SetResources([]datadogV2.MetricResource{
			{
				Name: datadog.PtrString(hostname),
				Type: datadog.PtrString("host"),
			},
		})
	}
	return metrics
}

// TagsFromBuildInfo returns a list of tags derived from buildInfo to be used when creating metrics.
func TagsFromBuildInfo(buildInfo component.BuildInfo) []string {
	var tags []string
	if buildInfo.Version != "" {
		tags = append(tags, "version:"+buildInfo.Version)
	}
	if buildInfo.Command != "" {
		tags = append(tags, "command:"+buildInfo.Command)
	}
	return tags
}
