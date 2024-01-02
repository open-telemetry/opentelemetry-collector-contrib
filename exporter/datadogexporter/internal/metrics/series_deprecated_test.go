// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestNewZorkianMetric(t *testing.T) {
	name := "test.metric"
	ts := uint64(1e9)
	value := 2.0
	tags := []string{"tag:value"}

	metric := newZorkianMetric(name, ts, value, tags)

	assert.Equal(t, "test.metric", *metric.Metric)
	// Assert timestamp conversion from uint64 ns to float64 s
	assert.Equal(t, 1.0, *metric.Points[0][0])
	// Assert value
	assert.Equal(t, 2.0, *metric.Points[0][1])
	// Assert tags
	assert.Equal(t, []string{"tag:value"}, metric.Tags)
}

func TestNewZorkianType(t *testing.T) {
	name := "test.metric"
	ts := uint64(1e9)
	value := 2.0
	tags := []string{"tag:value"}

	gauge := NewZorkianGauge(name, ts, value, tags)
	assert.Equal(t, gauge.GetType(), string(Gauge))

	count := NewZorkianCount(name, ts, value, tags)
	assert.Equal(t, count.GetType(), string(Count))

}

func TestDefaultZorkianMetrics(t *testing.T) {
	buildInfo := component.BuildInfo{
		Version: "1.0",
		Command: "otelcontribcol",
	}

	ms := DefaultZorkianMetrics("metrics", "test-host", uint64(2e9), buildInfo)

	assert.Equal(t, "otel.datadog_exporter.metrics.running", *ms[0].Metric)
	// Assert metrics list length (should be 1)
	assert.Equal(t, 1, len(ms))
	// Assert timestamp
	assert.Equal(t, 2.0, *ms[0].Points[0][0])
	// Assert value (should always be 1.0)
	assert.Equal(t, 1.0, *ms[0].Points[0][1])
	// Assert hostname tag is set
	assert.Equal(t, "test-host", *ms[0].Host)
	// Assert no other tags are set
	assert.ElementsMatch(t, []string{"version:1.0", "command:otelcontribcol"}, ms[0].Tags)
}
