// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
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
	}
	return metrics
}

// GatewayUsageGauge creates a gauge metric to report if there is a gateway
func GatewayUsageGauge(timestamp uint64, hostname string, tags []string, gatewayUsage *attributes.GatewayUsage) datadogV2.MetricSeries {
	series := NewGauge("datadog.otel.gateway", timestamp, gatewayUsage.Gauge(), tags)
	series.SetResources([]datadogV2.MetricResource{
		{
			Name: datadog.PtrString(hostname),
			Type: datadog.PtrString("host"),
		},
	})
	return series
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
