// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	otlpmetrics "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/component"
)

// newMetricSeries creates a new Datadog metric series given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func newMetricSeries(name string, ts uint64, value float64, tags []string) datadogV2.MetricSeries {
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
	return metric
}

// NewMetric creates a new DatadogV2 metric given a name, a type, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewMetric(name string, dt datadogV2.MetricIntakeType, ts uint64, value float64, tags []string) datadogV2.MetricSeries {
	metric := newMetricSeries(name, ts, value, tags)
	metric.SetType(dt)
	return metric
}

// NewGauge creates a new DatadogV2 Gauge metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewGauge(name string, ts uint64, value float64, tags []string) datadogV2.MetricSeries {
	return NewMetric(name, datadogV2.METRICINTAKETYPE_GAUGE, ts, value, tags)
}

// NewCount creates a new DatadogV2 count metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewCount(name string, ts uint64, value float64, tags []string) datadogV2.MetricSeries {
	return NewMetric(name, datadogV2.METRICINTAKETYPE_COUNT, ts, value, tags)
}

// DefaultMetrics creates built-in metrics to report that an exporter is running
func DefaultMetrics(exporterType string, hostname string, timestamp uint64, tags []string) []datadogV2.MetricSeries {
	metrics := []datadogV2.MetricSeries{
		NewGauge(fmt.Sprintf("otel.datadog_exporter.%s.running", exporterType), timestamp, 1.0, tags),
	}
	for i := range metrics {
		metrics[i].SetResources([]datadogV2.MetricResource{
			{
				Name: datadog.PtrString(hostname),
				Type: datadog.PtrString("host"),
			},
		})

		// datadog-api-client-go does not support `origin_product`, `origin_sub_product` or `origin_product_detail`.
		// We add them as 'AdditionalProperties'. This is undocumented; it adds the fields to the JSON output as-is:
		// https://github.com/DataDog/datadog-api-client-go/blob/f692d3/api/datadogV2/model_metric_origin.go#L153-L155
		// To make things more fun, the `MetricsOrigin` struct has references to deprecated `product` and `service` fields,
		// so to avoid sending these we use `MetricMetadata`'s `AdditionalProperties` field instead of `MetricsOrigin`'s.
		// Should be kept in sync with `Consumer.ConsumeTimeSeries`.
		metrics[i].SetMetadata(datadogV2.MetricMetadata{
			AdditionalProperties: map[string]any{
				"origin": map[string]any{
					"origin_product":        int(otlpmetrics.OriginProductDatadogExporter),
					"origin_sub_product":    int(otlpmetrics.OriginSubProductOTLP),
					"origin_product_detail": int(otlpmetrics.OriginProductDetailUnknown),
				}},
		})
	}
	return metrics
}

// TagsFromBuildInfo returns a list of tags derived from buildInfo to be used when creating metrics
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
