// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestNewMetricSeries(t *testing.T) {
	name := "test.metric"
	ts := uint64(1e9)
	value := 2.0
	tags := []string{"tag:value"}

	metric := newMetricSeries(name, ts, value, tags)

	assert.Equal(t, "test.metric", metric.Metric)
	// Assert timestamp conversion from uint64 ns to int64 s
	assert.Equal(t, int64(1), *metric.Points[0].Timestamp)
	// Assert value
	assert.Equal(t, 2.0, *metric.Points[0].Value)
	// Assert tags
	assert.Equal(t, []string{"tag:value"}, metric.Tags)
}

func TestNewType(t *testing.T) {
	name := "test.metric"
	ts := uint64(1e9)
	value := 2.0
	tags := []string{"tag:value"}

	gauge := NewGauge(name, ts, value, tags)
	assert.Equal(t, gauge.GetType(), datadogV2.METRICINTAKETYPE_GAUGE)

	count := NewCount(name, ts, value, tags)
	assert.Equal(t, count.GetType(), datadogV2.METRICINTAKETYPE_COUNT)
}

func TestDefaultMetrics(t *testing.T) {
	buildInfo := component.BuildInfo{
		Version: "1.0",
		Command: "otelcontribcol",
	}

	ms := DefaultMetrics("metrics", "test-host", uint64(2e9), buildInfo)

	assert.Equal(t, "otel.datadog_exporter.metrics.running", ms[0].Metric)
	// Assert metrics list length (should be 1)
	assert.Equal(t, 1, len(ms))
	// Assert timestamp
	assert.Equal(t, int64(2), *ms[0].Points[0].Timestamp)
	// Assert value (should always be 1.0)
	assert.Equal(t, 1.0, *ms[0].Points[0].Value)
	// Assert hostname tag is set
	assert.Equal(t, "test-host", *ms[0].Resources[0].Name)
	// Assert no other tags are set
	assert.ElementsMatch(t, []string{"version:1.0", "command:otelcontribcol"}, ms[0].Tags)
}
