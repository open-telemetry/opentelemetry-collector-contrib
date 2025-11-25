// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/metrics"

import (
	"github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-agent/pkg/tagset"
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

		// Set metric origin to identify these metrics as coming from Datadog Extension
		// Using OriginProduct = datadog_exporter (19)
		origin := datadogV2.NewMetricOrigin()
		origin.SetProduct(19) // datadog_exporter

		metadata := datadogV2.NewMetricMetadata()
		metadata.SetOrigin(*origin)
		metrics[i].SetMetadata(*metadata)
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

// ConvertToAgentSeries converts datadogV2.MetricSeries to the agent's series.Series format
// for use with the serializer component
func ConvertToAgentSeries(v2Series []datadogV2.MetricSeries, hostname string) metrics.Series {
	var agentSeries metrics.Series

	for _, metric := range v2Series {
		for _, point := range metric.Points {
			if point.Timestamp == nil || point.Value == nil {
				continue
			}

			// Convert metric type to APIMetricType
			var metricType metrics.APIMetricType
			if metric.Type != nil {
				switch *metric.Type {
				case datadogV2.METRICINTAKETYPE_GAUGE:
					metricType = metrics.APIGaugeType
				case datadogV2.METRICINTAKETYPE_COUNT:
					metricType = metrics.APICountType
				case datadogV2.METRICINTAKETYPE_RATE:
					metricType = metrics.APIRateType
				default:
					metricType = metrics.APIGaugeType
				}
			} else {
				metricType = metrics.APIGaugeType
			}

			// Create agent serie
			// Note: Tags field expects a CompositeTags type
			serie := &metrics.Serie{
				Name:           metric.Metric,
				Points:         []metrics.Point{{Ts: float64(*point.Timestamp), Value: *point.Value}},
				Tags:           tagset.NewCompositeTags(metric.Tags, nil),
				Host:           hostname,
				MType:          metricType,
				SourceTypeName: "otel.datadog_extension",
				Source:         metrics.MetricSourceOpenTelemetryCollectorUnknown,
			}

			agentSeries = append(agentSeries, serie)
		}
	}

	return agentSeries
}
