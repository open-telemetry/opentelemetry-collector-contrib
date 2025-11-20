// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

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
