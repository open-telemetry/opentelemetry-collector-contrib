// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestDefaultMetrics(t *testing.T) {
	hostname := "test-host"
	timestamp := uint64(1234567890000000000) // Unix nano timestamp
	tags := []string{"version:1.0.0", "command:otelcol"}

	metrics := DefaultMetrics(hostname, timestamp, tags)

	require.Len(t, metrics, 1, "should return one metric")

	metric := metrics[0]
	assert.Equal(t, "otel.datadog_extension.running", metric.Metric, "metric name should match")
	assert.Equal(t, tags, metric.Tags, "tags should match")

	// Verify timestamp conversion (nanoseconds to seconds)
	require.Len(t, metric.Points, 1, "should have one data point")
	expectedTimestamp := int64(1234567890) // Converted to Unix seconds
	assert.Equal(t, expectedTimestamp, *metric.Points[0].Timestamp, "timestamp should be converted to seconds")
	assert.Equal(t, 1.0, *metric.Points[0].Value, "value should be 1.0")

	// Verify resources
	require.NotNil(t, metric.Resources, "resources should be set")
	require.Len(t, metric.Resources, 1, "should have one resource")
	resource := metric.Resources[0]
	assert.Equal(t, hostname, *resource.Name, "resource name should match hostname")
	assert.Equal(t, "host", *resource.Type, "resource type should be host")

	// Verify metadata and origin
	require.NotNil(t, metric.Metadata, "metadata should be set")
	require.NotNil(t, metric.Metadata.Origin, "origin should be set")
	assert.Equal(t, int32(19), *metric.Metadata.Origin.Product, "product should be 19 (datadog_exporter)")
}

func TestNewGauge(t *testing.T) {
	name := "test.metric"
	timestamp := uint64(1234567890000000000)
	value := 42.5
	tags := []string{"env:prod", "service:test"}

	metric := NewGauge(name, timestamp, value, tags)

	assert.Equal(t, name, metric.Metric, "metric name should match")
	assert.Equal(t, tags, metric.Tags, "tags should match")

	require.Len(t, metric.Points, 1, "should have one data point")
	expectedTimestamp := int64(1234567890)
	assert.Equal(t, expectedTimestamp, *metric.Points[0].Timestamp, "timestamp should be converted to seconds")
	assert.Equal(t, value, *metric.Points[0].Value, "value should match")
}

func TestTagsFromBuildInfo(t *testing.T) {
	t.Run("with version and command", func(t *testing.T) {
		buildInfo := component.BuildInfo{
			Version: "1.2.3",
			Command: "otelcol-contrib",
		}

		tags := TagsFromBuildInfo(buildInfo)

		require.Len(t, tags, 2, "should have two tags")
		assert.Contains(t, tags, "version:1.2.3", "should contain version tag")
		assert.Contains(t, tags, "command:otelcol-contrib", "should contain command tag")
	})

	t.Run("with empty values", func(t *testing.T) {
		buildInfo := component.BuildInfo{
			Version: "",
			Command: "",
		}

		tags := TagsFromBuildInfo(buildInfo)

		assert.Empty(t, tags, "should have no tags when values are empty")
	})

	t.Run("with only version", func(t *testing.T) {
		buildInfo := component.BuildInfo{
			Version: "1.2.3",
			Command: "",
		}

		tags := TagsFromBuildInfo(buildInfo)

		require.Len(t, tags, 1, "should have one tag")
		assert.Contains(t, tags, "version:1.2.3", "should contain version tag")
	})
}

func TestConvertToAgentSeries(t *testing.T) {
	t.Run("converts gauge metric correctly", func(t *testing.T) {
		timestamp := int64(1234567890)
		value := 42.5
		hostname := "test-host"
		gaugeType := datadogV2.METRICINTAKETYPE_GAUGE

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.metric",
				Type:   &gaugeType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp,
						Value:     &value,
					},
				},
				Tags: []string{"env:prod", "service:test"},
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		require.Len(t, agentSeries, 1, "should return one serie")
		serie := agentSeries[0]

		assert.Equal(t, "test.metric", serie.Name, "metric name should match")
		assert.Equal(t, hostname, serie.Host, "hostname should match")
		assert.Equal(t, metrics.APIGaugeType, serie.MType, "metric type should be gauge")
		assert.Equal(t, "otel.datadog_extension", serie.SourceTypeName, "source type name should be set")
		assert.Equal(t, metrics.MetricSourceOpenTelemetryCollectorUnknown, serie.Source, "metric source should be set to OpenTelemetry Collector")

		// Verify points
		require.Len(t, serie.Points, 1, "should have one point")
		assert.Equal(t, float64(timestamp), serie.Points[0].Ts, "timestamp should match")
		assert.Equal(t, value, serie.Points[0].Value, "value should match")

		// Verify tags
		assert.Equal(t, 2, serie.Tags.Len(), "should have two tags")
		assert.True(t, serie.Tags.Find(func(tag string) bool { return tag == "env:prod" }), "should contain env tag")
		assert.True(t, serie.Tags.Find(func(tag string) bool { return tag == "service:test" }), "should contain service tag")
	})

	t.Run("converts count metric correctly", func(t *testing.T) {
		timestamp := int64(1234567890)
		value := 100.0
		hostname := "test-host"
		countType := datadogV2.METRICINTAKETYPE_COUNT

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.count",
				Type:   &countType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp,
						Value:     &value,
					},
				},
				Tags: []string{},
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		require.Len(t, agentSeries, 1, "should return one serie")
		assert.Equal(t, metrics.APICountType, agentSeries[0].MType, "metric type should be count")
	})

	t.Run("converts rate metric correctly", func(t *testing.T) {
		timestamp := int64(1234567890)
		value := 5.5
		hostname := "test-host"
		rateType := datadogV2.METRICINTAKETYPE_RATE

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.rate",
				Type:   &rateType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp,
						Value:     &value,
					},
				},
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		require.Len(t, agentSeries, 1, "should return one serie")
		assert.Equal(t, metrics.APIRateType, agentSeries[0].MType, "metric type should be rate")
	})

	t.Run("defaults to gauge when type is nil", func(t *testing.T) {
		timestamp := int64(1234567890)
		value := 42.0
		hostname := "test-host"

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.metric",
				Type:   nil, // No type specified
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp,
						Value:     &value,
					},
				},
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		require.Len(t, agentSeries, 1, "should return one serie")
		assert.Equal(t, metrics.APIGaugeType, agentSeries[0].MType, "metric type should default to gauge")
	})

	t.Run("handles multiple points from single metric", func(t *testing.T) {
		timestamp1 := int64(1234567890)
		value1 := 10.0
		timestamp2 := int64(1234567900)
		value2 := 20.0
		hostname := "test-host"
		gaugeType := datadogV2.METRICINTAKETYPE_GAUGE

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.metric",
				Type:   &gaugeType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp1,
						Value:     &value1,
					},
					{
						Timestamp: &timestamp2,
						Value:     &value2,
					},
				},
				Tags: []string{"env:test"},
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		require.Len(t, agentSeries, 2, "should return two series (one per point)")
		assert.Equal(t, "test.metric", agentSeries[0].Name, "first serie name should match")
		assert.Equal(t, "test.metric", agentSeries[1].Name, "second serie name should match")
		assert.Equal(t, float64(timestamp1), agentSeries[0].Points[0].Ts, "first timestamp should match")
		assert.Equal(t, float64(timestamp2), agentSeries[1].Points[0].Ts, "second timestamp should match")
	})

	t.Run("handles multiple metrics", func(t *testing.T) {
		timestamp := int64(1234567890)
		value1 := 10.0
		value2 := 20.0
		hostname := "test-host"
		gaugeType := datadogV2.METRICINTAKETYPE_GAUGE
		countType := datadogV2.METRICINTAKETYPE_COUNT

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.metric1",
				Type:   &gaugeType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp,
						Value:     &value1,
					},
				},
			},
			{
				Metric: "test.metric2",
				Type:   &countType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp,
						Value:     &value2,
					},
				},
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		require.Len(t, agentSeries, 2, "should return two series")
		assert.Equal(t, "test.metric1", agentSeries[0].Name, "first metric name should match")
		assert.Equal(t, "test.metric2", agentSeries[1].Name, "second metric name should match")
		assert.Equal(t, metrics.APIGaugeType, agentSeries[0].MType, "first metric type should be gauge")
		assert.Equal(t, metrics.APICountType, agentSeries[1].MType, "second metric type should be count")
	})

	t.Run("skips points with nil timestamp", func(t *testing.T) {
		value := 42.0
		hostname := "test-host"
		gaugeType := datadogV2.METRICINTAKETYPE_GAUGE

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.metric",
				Type:   &gaugeType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: nil, // Missing timestamp
						Value:     &value,
					},
				},
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		assert.Empty(t, agentSeries, "should return empty series when timestamp is nil")
	})

	t.Run("skips points with nil value", func(t *testing.T) {
		timestamp := int64(1234567890)
		hostname := "test-host"
		gaugeType := datadogV2.METRICINTAKETYPE_GAUGE

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.metric",
				Type:   &gaugeType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp,
						Value:     nil, // Missing value
					},
				},
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		assert.Empty(t, agentSeries, "should return empty series when value is nil")
	})

	t.Run("handles empty input", func(t *testing.T) {
		hostname := "test-host"

		agentSeries := ConvertToAgentSeries([]datadogV2.MetricSeries{}, hostname)

		assert.Empty(t, agentSeries, "should return empty series for empty input")
	})

	t.Run("handles empty tags", func(t *testing.T) {
		timestamp := int64(1234567890)
		value := 42.0
		hostname := "test-host"
		gaugeType := datadogV2.METRICINTAKETYPE_GAUGE

		v2Series := []datadogV2.MetricSeries{
			{
				Metric: "test.metric",
				Type:   &gaugeType,
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: &timestamp,
						Value:     &value,
					},
				},
				Tags: []string{}, // Empty tags
			},
		}

		agentSeries := ConvertToAgentSeries(v2Series, hostname)

		require.Len(t, agentSeries, 1, "should return one serie")
		assert.Equal(t, 0, agentSeries[0].Tags.Len(), "should have no tags")
	})
}
