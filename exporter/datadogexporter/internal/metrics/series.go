// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"fmt"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
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
func NewMetric(name string, dt datadogV2.MetricIntakeType, ts uint64, interval int64, value float64, tags []string) datadogV2.MetricSeries {
	metric := newMetricSeries(name, ts, value, tags)
	metric.SetType(dt)
	metric.SetInterval(interval)
	return metric
}

// NewGauge creates a new DatadogV2 Gauge metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewGauge(name string, ts uint64, interval int64, value float64, tags []string) datadogV2.MetricSeries {
	return NewMetric(name, datadogV2.METRICINTAKETYPE_GAUGE, ts, interval, value, tags)
}

// NewCount creates a new DatadogV2 count metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewCount(name string, ts uint64, interval int64, value float64, tags []string) datadogV2.MetricSeries {
	return NewMetric(name, datadogV2.METRICINTAKETYPE_COUNT, ts, interval, value, tags)
}

func newDefaultMetrics(exporterType string, timestamp uint64, tags []string) []datadogV2.MetricSeries {
	return []datadogV2.MetricSeries{
		NewGauge(fmt.Sprintf("otel.datadog_exporter.%s.running", exporterType), timestamp, 0, 1.0, tags),
	}
}

func setHostResource(metric *datadogV2.MetricSeries, hostname string) {
	metric.SetResources([]datadogV2.MetricResource{
		{
			Name: datadog.PtrString(hostname),
			Type: datadog.PtrString("host"),
		},
	})
}

func setHostResources(metrics []datadogV2.MetricSeries, hostname string) {
	for i := range metrics {
		setHostResource(&metrics[i], hostname)
	}
}

// DefaultMetrics creates built-in metrics to report that an exporter is running
func DefaultMetrics(exporterType, hostname string, timestamp uint64, tags []string) []datadogV2.MetricSeries {
	metrics := newDefaultMetrics(exporterType, timestamp, tags)
	setHostResources(metrics, hostname)
	return metrics
}

func newFargateMetrics(timestamp uint64, tags []string) []datadogV2.MetricSeries {
	return []datadogV2.MetricSeries{
		NewGauge("otel.datadog_exporter.metrics.running.fargate", timestamp, 0, 1.0, tags),
	}
}

// FargateMetrics creates built-in metrics to report that a Fargate exporter is running.
func FargateMetrics(timestamp uint64, tags []string) []datadogV2.MetricSeries {
	metrics := newFargateMetrics(timestamp, tags)
	setHostResources(metrics, "")
	return metrics
}

func newGatewayUsageGauge(timestamp uint64, tags []string, gatewayUsage *attributes.GatewayUsage) datadogV2.MetricSeries {
	return NewGauge("datadog.otel.gateway", timestamp, 0, gatewayUsage.Gauge(), tags)
}

// GatewayUsageGauge creates a gauge metric to report if there is a gateway
func GatewayUsageGauge(timestamp uint64, hostname string, tags []string, gatewayUsage *attributes.GatewayUsage) datadogV2.MetricSeries {
	metric := newGatewayUsageGauge(timestamp, tags, gatewayUsage)
	setHostResource(&metric, hostname)
	return metric
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
