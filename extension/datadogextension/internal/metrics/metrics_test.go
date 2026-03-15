// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestCreateLivenessSerie(t *testing.T) {
	hostname := "test-host"
	timestamp := uint64(1234567890000000000) // Unix nano timestamp
	tags := []string{"version:1.0.0", "command:otelcol", "hostname_source:config"}

	serie := CreateLivenessSerie(hostname, timestamp, tags)

	require.NotNil(t, serie, "should return a serie")

	// Verify basic properties
	assert.Equal(t, "otel.datadog_extension.running", serie.Name, "metric name should match")
	assert.Equal(t, hostname, serie.Host, "hostname should match")
	assert.Equal(t, metrics.APIGaugeType, serie.MType, "metric type should be gauge")
	assert.Equal(t, "otel.datadog_extension", serie.SourceTypeName, "source type name should be set")
	assert.Equal(t, metrics.MetricSourceOpenTelemetryCollectorUnknown, serie.Source, "metric source should be set to OpenTelemetry Collector")

	// Verify points
	require.Len(t, serie.Points, 1, "should have one data point")
	expectedTimestamp := float64(1234567890) // Converted to Unix seconds
	assert.Equal(t, expectedTimestamp, serie.Points[0].Ts, "timestamp should be converted to seconds")
	assert.Equal(t, 1.0, serie.Points[0].Value, "value should be 1.0")

	// Verify tags
	assert.Equal(t, 3, serie.Tags.Len(), "should have three tags")
	assert.True(t, serie.Tags.Find(func(tag string) bool { return tag == "version:1.0.0" }), "should contain version tag")
	assert.True(t, serie.Tags.Find(func(tag string) bool { return tag == "command:otelcol" }), "should contain command tag")
	assert.True(t, serie.Tags.Find(func(tag string) bool { return tag == "hostname_source:config" }), "should contain hostname_source tag")
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
