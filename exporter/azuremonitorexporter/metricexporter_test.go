// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

/*
Contains tests for metricexporter.go and metric_to_envelopes.go
*/

import (
	"context"
	"testing"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Test onMetricData callback for the test metrics data
func TestExporterMetricDataCallback(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getAzureMonitorExporter(defaultConfig, mockTransportChannel)

	metrics := getTestMetrics()

	assert.NoError(t, exporter.consumeMetrics(context.Background(), metrics))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 5)
}

func TestDoubleGaugeEnvelopes(t *testing.T) {
	gaugeMetric := getDoubleTestGaugeMetric()
	dataPoint := getDataPoint(t, gaugeMetric)

	assert.Equal(t, "Gauge", dataPoint.Name)
	assert.Equal(t, float64(1), dataPoint.Value)
	assert.Equal(t, 1, dataPoint.Count)
	assert.Equal(t, contracts.Measurement, dataPoint.Kind)
}

func TestIntGaugeEnvelopes(t *testing.T) {
	gaugeMetric := getIntTestGaugeMetric()
	dataPoint := getDataPoint(t, gaugeMetric)

	assert.Equal(t, "Gauge", dataPoint.Name)
	assert.Equal(t, float64(1), dataPoint.Value)
	assert.Equal(t, 1, dataPoint.Count)
	assert.Equal(t, contracts.Measurement, dataPoint.Kind)
}

func TestDoubleSumEnvelopes(t *testing.T) {
	sumMetric := getDoubleTestSumMetric()
	dataPoint := getDataPoint(t, sumMetric)

	assert.Equal(t, "Sum", dataPoint.Name)
	assert.Equal(t, float64(2), dataPoint.Value)
	assert.Equal(t, 1, dataPoint.Count)
	assert.Equal(t, contracts.Measurement, dataPoint.Kind)
}

func TestIntSumEnvelopes(t *testing.T) {
	sumMetric := getIntTestSumMetric()
	dataPoint := getDataPoint(t, sumMetric)

	assert.Equal(t, "Sum", dataPoint.Name)
	assert.Equal(t, float64(2), dataPoint.Value)
	assert.Equal(t, 1, dataPoint.Count)
	assert.Equal(t, contracts.Measurement, dataPoint.Kind)
}

func TestHistogramEnvelopes(t *testing.T) {
	histogramMetric := getTestHistogramMetric()
	dataPoint := getDataPoint(t, histogramMetric)

	assert.Equal(t, "Histogram", dataPoint.Name)
	assert.Equal(t, float64(3), dataPoint.Value)
	assert.Equal(t, 3, dataPoint.Count)
	assert.Equal(t, float64(0), dataPoint.Min)
	assert.Equal(t, float64(2), dataPoint.Max)
	assert.Equal(t, contracts.Aggregation, dataPoint.Kind)
}

func TestExponentialHistogramEnvelopes(t *testing.T) {
	exponentialHistogramMetric := getTestExponentialHistogramMetric()
	dataPoint := getDataPoint(t, exponentialHistogramMetric)

	assert.Equal(t, "ExponentialHistogram", dataPoint.Name)
	assert.Equal(t, float64(4), dataPoint.Value)
	assert.Equal(t, 4, dataPoint.Count)
	assert.Equal(t, float64(1), dataPoint.Min)
	assert.Equal(t, float64(3), dataPoint.Max)
	assert.Equal(t, contracts.Aggregation, dataPoint.Kind)
}

func TestSummaryEnvelopes(t *testing.T) {
	summaryMetric := getTestSummaryMetric()
	dataPoint := getDataPoint(t, summaryMetric)

	assert.Equal(t, "Summary", dataPoint.Name)
	assert.Equal(t, float64(5), dataPoint.Value)
	assert.Equal(t, 5, dataPoint.Count)
	assert.Equal(t, contracts.Aggregation, dataPoint.Kind)
}

func getDataPoint(tb testing.TB, metric pmetric.Metric) *contracts.DataPoint {
	envelopes := getMetricPacker().MetricToEnvelopes(metric, getResource(), getScope())
	require.Len(tb, envelopes, 1)
	envelope := envelopes[0]
	require.NotNil(tb, envelope)

	assert.NotNil(tb, envelope.Tags)
	assert.Contains(tb, envelope.Tags[contracts.InternalSdkVersion], "otelc-")
	assert.NotNil(tb, envelope.Time)

	require.NotNil(tb, envelope.Data)
	envelopeData := envelope.Data.(*contracts.Data)
	assert.Equal(tb, "MetricData", envelopeData.BaseType)

	require.NotNil(tb, envelopeData.BaseData)

	metricData := envelopeData.BaseData.(*contracts.MetricData)

	require.Len(tb, metricData.Metrics, 1)

	dataPoint := metricData.Metrics[0]
	require.NotNil(tb, dataPoint)

	actualProperties := metricData.Properties
	require.Equal(tb, "10", actualProperties["int_attribute"])
	require.Equal(tb, "str_value", actualProperties["str_attribute"])
	require.Equal(tb, "true", actualProperties["bool_attribute"])
	require.Equal(tb, "1.2", actualProperties["double_attribute"])

	return dataPoint
}

func getAzureMonitorExporter(config *Config, transportChannel appinsights.TelemetryChannel) *azureMonitorExporter {
	return &azureMonitorExporter{
		config,
		transportChannel,
		zap.NewNop(),
		newMetricPacker(zap.NewNop()),
	}
}

func getMetricPacker() *metricPacker {
	return newMetricPacker(zap.NewNop())
}

func getTestMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	resourceMetricsSlice := metrics.ResourceMetrics()
	resourceMetric := resourceMetricsSlice.AppendEmpty()
	scopeMetricsSlice := resourceMetric.ScopeMetrics()
	scopeMetrics := scopeMetricsSlice.AppendEmpty()
	metricSlice := scopeMetrics.Metrics()

	metric := metricSlice.AppendEmpty()
	gaugeMetric := getDoubleTestGaugeMetric()
	gaugeMetric.CopyTo(metric)

	metric = metricSlice.AppendEmpty()
	sumMetric := getIntTestSumMetric()
	sumMetric.CopyTo(metric)

	metric = metricSlice.AppendEmpty()
	histogramMetric := getTestHistogramMetric()
	histogramMetric.CopyTo(metric)

	metric = metricSlice.AppendEmpty()
	exponentialHistogramMetric := getTestExponentialHistogramMetric()
	exponentialHistogramMetric.CopyTo(metric)

	metric = metricSlice.AppendEmpty()
	summaryMetric := getTestSummaryMetric()
	summaryMetric.CopyTo(metric)

	return metrics
}

func getDoubleTestGaugeMetric() pmetric.Metric {
	return getTestGaugeMetric(func(datapoint pmetric.NumberDataPoint) {
		datapoint.SetDoubleValue(1)
	})
}

func getIntTestGaugeMetric() pmetric.Metric {
	return getTestGaugeMetric(func(datapoint pmetric.NumberDataPoint) {
		datapoint.SetIntValue(1)
	})
}

func getTestGaugeMetric(modify func(pmetric.NumberDataPoint)) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("Gauge")
	metric.SetEmptyGauge()
	datapoints := metric.Gauge().DataPoints()
	datapoint := datapoints.AppendEmpty()
	setDefaultTestAttributes(datapoint.Attributes())
	modify(datapoint)
	return metric
}

func getDoubleTestSumMetric() pmetric.Metric {
	return getTestSumMetric(func(datapoint pmetric.NumberDataPoint) {
		datapoint.SetDoubleValue(2)
	})
}

func getIntTestSumMetric() pmetric.Metric {
	return getTestSumMetric(func(datapoint pmetric.NumberDataPoint) {
		datapoint.SetIntValue(2)
	})
}

func getTestSumMetric(modify func(pmetric.NumberDataPoint)) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("Sum")
	metric.SetEmptySum()
	datapoints := metric.Sum().DataPoints()
	datapoint := datapoints.AppendEmpty()
	setDefaultTestAttributes(datapoint.Attributes())
	modify(datapoint)
	return metric
}

func getTestHistogramMetric() pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("Histogram")
	metric.SetEmptyHistogram()
	datapoints := metric.Histogram().DataPoints()
	datapoint := datapoints.AppendEmpty()
	datapoint.SetSum(3)
	datapoint.SetCount(3)
	datapoint.SetMin(0)
	datapoint.SetMax(2)
	setDefaultTestAttributes(datapoint.Attributes())
	return metric
}

func getTestExponentialHistogramMetric() pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("ExponentialHistogram")
	metric.SetEmptyExponentialHistogram()
	datapoints := metric.ExponentialHistogram().DataPoints()
	datapoint := datapoints.AppendEmpty()
	datapoint.SetSum(4)
	datapoint.SetCount(4)
	datapoint.SetMin(1)
	datapoint.SetMax(3)
	setDefaultTestAttributes(datapoint.Attributes())
	return metric
}

func getTestSummaryMetric() pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("Summary")
	metric.SetEmptySummary()
	datapoints := metric.Summary().DataPoints()
	datapoint := datapoints.AppendEmpty()
	datapoint.SetSum(5)
	datapoint.SetCount(5)
	setDefaultTestAttributes(datapoint.Attributes())
	return metric
}

func setDefaultTestAttributes(attributeMap pcommon.Map) {
	attributeMap.PutInt("int_attribute", 10)
	attributeMap.PutStr("str_attribute", "str_value")
	attributeMap.PutBool("bool_attribute", true)
	attributeMap.PutDouble("double_attribute", 1.2)
}
