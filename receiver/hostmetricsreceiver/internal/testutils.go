// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func AssertDescriptorEqual(t *testing.T, expected pmetric.Metric, actual pmetric.Metric) {
	assert.Equal(t, expected.Name(), actual.Name())
	assert.Equal(t, expected.Description(), actual.Description())
	assert.Equal(t, expected.Unit(), actual.Unit())
	assert.Equal(t, expected.Type(), actual.Type())
}

func AssertSumMetricHasAttributeValue(t *testing.T, metric pmetric.Metric, index int, labelName string, expectedVal pcommon.Value) {
	val, ok := metric.Sum().DataPoints().At(index).Attributes().Get(labelName)
	assert.Truef(t, ok, "Missing attribute %q in metric %q", labelName, metric.Name())
	assert.Equal(t, expectedVal, val)
}

func AssertSumMetricHasAttribute(t *testing.T, metric pmetric.Metric, index int, labelName string) {
	_, ok := metric.Sum().DataPoints().At(index).Attributes().Get(labelName)
	assert.Truef(t, ok, "Missing attribute %q in metric %q", labelName, metric.Name())
}

func AssertSumMetricStartTimeEquals(t *testing.T, metric pmetric.Metric, startTime pcommon.Timestamp) {
	ddps := metric.Sum().DataPoints()
	for i := 0; i < ddps.Len(); i++ {
		require.Equal(t, startTime, ddps.At(i).StartTimestamp())
	}
}

func AssertGaugeMetricHasAttributeValue(t *testing.T, metric pmetric.Metric, index int, labelName string, expectedVal pcommon.Value) {
	val, ok := metric.Gauge().DataPoints().At(index).Attributes().Get(labelName)
	assert.Truef(t, ok, "Missing attribute %q in metric %q", labelName, metric.Name())
	assert.Equal(t, expectedVal, val)
}

func AssertGaugeMetricHasAttribute(t *testing.T, metric pmetric.Metric, index int, labelName string) {
	_, ok := metric.Gauge().DataPoints().At(index).Attributes().Get(labelName)
	assert.Truef(t, ok, "Missing attribute %q in metric %q", labelName, metric.Name())
}

func AssertGaugeMetricStartTimeEquals(t *testing.T, metric pmetric.Metric, startTime pcommon.Timestamp) {
	ddps := metric.Gauge().DataPoints()
	for i := 0; i < ddps.Len(); i++ {
		require.Equal(t, startTime, ddps.At(i).StartTimestamp())
	}
}

func AssertSameTimeStampForAllMetrics(t *testing.T, metrics pmetric.MetricSlice) {
	AssertSameTimeStampForMetrics(t, metrics, 0, metrics.Len())
}

func AssertSameTimeStampForMetrics(t *testing.T, metrics pmetric.MetricSlice, startIdx, endIdx int) {
	var ts pcommon.Timestamp
	for i := startIdx; i < endIdx; i++ {
		metric := metrics.At(i)
		if metric.Type() == pmetric.MetricTypeSum {
			ddps := metric.Sum().DataPoints()
			for j := 0; j < ddps.Len(); j++ {
				if ts == 0 {
					ts = ddps.At(j).Timestamp()
				}
				require.Equalf(t, ts, ddps.At(j).Timestamp(), "metrics contained different end timestamp values")
			}
		}
	}
}

// AssertExpectedMetrics checks that metrics contains expected metrics and no other metrics.
// It also checks that each expected metric has at least one data point.
func AssertExpectedMetrics(t *testing.T, expectedMetrics map[string]bool, actualMetrics pmetric.Metrics) {
	unexpectedMetrics := map[string]bool{}

	metricsCount := 0
	totalDataPointCount := 0

	metricsSlice := actualMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < metricsSlice.Len(); i++ {
		metricsCount++
		metricsSlice.At(i)
		metric := metricsSlice.At(i)
		switch metric.Type() {
		case pmetric.MetricTypeSum:
			totalDataPointCount += metric.Sum().DataPoints().Len()
		case pmetric.MetricTypeGauge:
			totalDataPointCount += metric.Gauge().DataPoints().Len()
		case pmetric.MetricTypeHistogram:
			totalDataPointCount += metric.Histogram().DataPoints().Len()
		case pmetric.MetricTypeExponentialHistogram:
			totalDataPointCount += metric.ExponentialHistogram().DataPoints().Len()
		case pmetric.MetricTypeSummary:
			totalDataPointCount += metric.Summary().DataPoints().Len()
		default:
			assert.Fail(t, "Unexpected metric type")
		}

		metricName := metric.Name()
		if _, ok := expectedMetrics[metricName]; ok {
			expectedMetrics[metricName] = true
		} else {
			unexpectedMetrics[metricName] = true
		}
	}

	// Check that we have some metrics and data points
	assert.Positive(t, metricsCount, "No metrics were collected")
	assert.Positive(t, totalDataPointCount, "No data points were collected")

	// Check that each expected metric has at least one data point
	for metricName, hasDataPoints := range expectedMetrics {
		assert.True(t, hasDataPoints, "Metric %s has no data points", metricName)
	}
	// Check that no unexpected metrics were collected
	for metricName := range unexpectedMetrics {
		assert.Fail(t, "Unexpected metric was collected", metricName)
	}
}
