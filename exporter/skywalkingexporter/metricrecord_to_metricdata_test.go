// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	metricpb "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

func TestMetricDataToLogService(t *testing.T) {
	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty() // Add an empty ResourceMetrics
	rm := md.ResourceMetrics().AppendEmpty()

	rm.Resource().Attributes().PutStr("labelB", "valueB")
	rm.Resource().Attributes().PutStr("labelA", "valueA")
	rm.Resource().Attributes().PutStr("a", "b")
	ilms := rm.ScopeMetrics()
	ilms.AppendEmpty() // Add an empty ScopeMetrics
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()

	badNameMetric := metrics.AppendEmpty()
	badNameMetric.SetName("")

	noneMetric := metrics.AppendEmpty()
	noneMetric.SetName("none")

	intGaugeMetric := metrics.AppendEmpty()
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.SetEmptyGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.Attributes().PutStr("innerLabel", "innerValue")
	intGaugeDataPoint.Attributes().PutStr("testa", "test")
	intGaugeDataPoint.SetIntValue(10)
	intGaugeDataPoint.SetTimestamp(pcommon.Timestamp(100_000_000))

	doubleGaugeMetric := metrics.AppendEmpty()
	doubleGaugeMetric.SetName("double_gauge")
	doubleGauge := doubleGaugeMetric.SetEmptyGauge()
	doubleGaugeDataPoints := doubleGauge.DataPoints()
	doubleGaugeDataPoint := doubleGaugeDataPoints.AppendEmpty()
	doubleGaugeDataPoint.Attributes().PutStr("innerLabel", "innerValue")
	doubleGaugeDataPoint.SetDoubleValue(10.1)
	doubleGaugeDataPoint.SetTimestamp(pcommon.Timestamp(100_000_000))

	intSumMetric := metrics.AppendEmpty()
	intSumMetric.SetName("int_sum")
	intSum := intSumMetric.SetEmptySum()
	intSumDataPoints := intSum.DataPoints()
	intSumDataPoint := intSumDataPoints.AppendEmpty()
	intSumDataPoint.Attributes().PutStr("innerLabel", "innerValue")
	intSumDataPoint.SetIntValue(11)
	intSumDataPoint.SetTimestamp(pcommon.Timestamp(100_000_000))

	doubleSumMetric := metrics.AppendEmpty()
	doubleSumMetric.SetName("double_sum")
	doubleSum := doubleSumMetric.SetEmptySum()
	doubleSumDataPoints := doubleSum.DataPoints()
	doubleSumDataPoint := doubleSumDataPoints.AppendEmpty()
	doubleSumDataPoint.Attributes().PutStr("innerLabel", "innerValue")
	doubleSumDataPoint.SetDoubleValue(10.1)
	doubleSumDataPoint.SetTimestamp(pcommon.Timestamp(100_000_000))

	doubleHistogramMetric := metrics.AppendEmpty()
	doubleHistogramMetric.SetName("double_$histogram")
	doubleHistogram := doubleHistogramMetric.SetEmptyHistogram()
	doubleHistogramDataPoints := doubleHistogram.DataPoints()
	doubleHistogramDataPoint := doubleHistogramDataPoints.AppendEmpty()
	doubleHistogramDataPoint.Attributes().PutStr("innerLabel", "innerValue")
	doubleHistogramDataPoint.Attributes().PutStr("innerLabelH", "innerValueH")
	doubleHistogramDataPoint.SetCount(5)
	doubleHistogramDataPoint.SetSum(10.1)
	doubleHistogramDataPoint.SetTimestamp(pcommon.Timestamp(100_000_000))
	doubleHistogramDataPoint.BucketCounts().FromRaw([]uint64{1, 2, 2})
	doubleHistogramDataPoint.ExplicitBounds().FromRaw([]float64{1, 2})

	doubleSummaryMetric := metrics.AppendEmpty()
	doubleSummaryMetric.SetName("double-summary")
	doubleSummary := doubleSummaryMetric.SetEmptySummary()
	doubleSummaryDataPoints := doubleSummary.DataPoints()
	doubleSummaryDataPoint := doubleSummaryDataPoints.AppendEmpty()
	doubleSummaryDataPoint.SetCount(2)
	doubleSummaryDataPoint.SetSum(10.1)
	doubleSummaryDataPoint.SetTimestamp(pcommon.Timestamp(100_000_000))
	doubleSummaryDataPoint.Attributes().PutStr("innerLabel", "innerValue")
	doubleSummaryDataPoint.Attributes().PutStr("innerLabelS", "innerValueS")
	quantileVal := doubleSummaryDataPoint.QuantileValues().AppendEmpty()
	quantileVal.SetValue(10.2)
	quantileVal.SetQuantile(0.9)
	quantileVal2 := doubleSummaryDataPoint.QuantileValues().AppendEmpty()
	quantileVal2.SetValue(10.5)
	quantileVal2.SetQuantile(0.95)

	gotLogs := metricsRecordToMetricData(md)

	assert.Equal(t, 11, len(gotLogs.MeterData))

	for i, meterData := range gotLogs.MeterData {
		assert.Equal(t, "valueB", searchMetricTag("labelB", meterData))
		assert.Equal(t, "valueA", searchMetricTag("labelA", meterData))
		assert.Equal(t, "b", searchMetricTag("a", meterData))
		assert.Equal(t, "innerValue", searchMetricTag("innerLabel", meterData))
		assert.Equal(t, defaultServiceName, meterData.GetService())
		assert.Equal(t, defaultServiceInstance, meterData.GetServiceInstance())
		switch i {
		case 0:
			assert.Equal(t, "int_gauge", meterData.GetSingleValue().GetName())
			assert.Equal(t, float64(10), meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
			assert.Equal(t, "test", searchMetricTag("testa", meterData))
		case 1:
			assert.Equal(t, "double_gauge", meterData.GetSingleValue().GetName())
			assert.Equal(t, 10.1, meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
		case 2:
			assert.Equal(t, "int_sum", meterData.GetSingleValue().GetName())
			assert.Equal(t, float64(11), meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
		case 3:
			assert.Equal(t, "double_sum", meterData.GetSingleValue().GetName())
			assert.Equal(t, 10.1, meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
		case 4:
			assert.Equal(t, "double_$histogram", meterData.GetHistogram().GetName())
			assert.Equal(t, 3, len(meterData.GetHistogram().GetValues()))
			assert.Equal(t, int64(1), meterData.GetHistogram().GetValues()[0].Count)
			assert.Equal(t, true, meterData.GetHistogram().GetValues()[0].IsNegativeInfinity)
			assert.Equal(t, int64(2), meterData.GetHistogram().GetValues()[1].Count)
			assert.Equal(t, false, meterData.GetHistogram().GetValues()[1].IsNegativeInfinity)
			assert.Equal(t, float64(1), meterData.GetHistogram().GetValues()[1].GetBucket())
			assert.Equal(t, int64(2), meterData.GetHistogram().GetValues()[2].Count)
			assert.Equal(t, false, meterData.GetHistogram().GetValues()[2].IsNegativeInfinity)
			assert.Equal(t, float64(2), meterData.GetHistogram().GetValues()[2].GetBucket())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
			assert.Equal(t, "innerValueH", searchMetricTag("innerLabelH", meterData))
		case 5:
			assert.Equal(t, "double_$histogram_sum", meterData.GetSingleValue().GetName())
			assert.Equal(t, 10.1, meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
			assert.Equal(t, "innerValueH", searchMetricTag("innerLabelH", meterData))
		case 6:
			assert.Equal(t, "double_$histogram_count", meterData.GetSingleValue().GetName())
			assert.Equal(t, float64(5), meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
			assert.Equal(t, "innerValueH", searchMetricTag("innerLabelH", meterData))
		case 7:
			assert.Equal(t, "double-summary", meterData.GetSingleValue().GetName())
			assert.Equal(t, 10.2, meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
			assert.Equal(t, "innerValueS", searchMetricTag("innerLabelS", meterData))
			assert.Equal(t, "0.9", searchMetricTag("quantile", meterData))
		case 8:
			assert.Equal(t, "double-summary", meterData.GetSingleValue().GetName())
			assert.Equal(t, 10.5, meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
			assert.Equal(t, "innerValueS", searchMetricTag("innerLabelS", meterData))
			assert.Equal(t, "0.95", searchMetricTag("quantile", meterData))
		case 9:
			assert.Equal(t, "double-summary_sum", meterData.GetSingleValue().GetName())
			assert.Equal(t, 10.1, meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
			assert.Equal(t, "innerValueS", searchMetricTag("innerLabelS", meterData))
		case 10:
			assert.Equal(t, "double-summary_count", meterData.GetSingleValue().GetName())
			assert.Equal(t, float64(2), meterData.GetSingleValue().GetValue())
			assert.Equal(t, int64(100), meterData.GetTimestamp())
			assert.Equal(t, "innerValueS", searchMetricTag("innerLabelS", meterData))
		}
	}
}

func searchMetricTag(name string, record *metricpb.MeterData) string {
	if _, ok := record.GetMetric().(*metricpb.MeterData_SingleValue); ok {
		for _, tag := range record.GetSingleValue().GetLabels() {
			if tag.Name == name {
				return tag.GetValue()
			}
		}
	}

	if _, ok := record.GetMetric().(*metricpb.MeterData_Histogram); ok {
		for _, tag := range record.GetHistogram().GetLabels() {
			if tag.Name == name {
				return tag.GetValue()
			}
		}
	}
	return ""
}
