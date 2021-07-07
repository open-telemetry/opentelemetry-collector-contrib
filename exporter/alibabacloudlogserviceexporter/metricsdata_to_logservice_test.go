// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alibabacloudlogserviceexporter

import (
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func TestMetricDataToLogService(t *testing.T) {
	logger := zap.NewNop()

	md := pdata.NewMetrics()
	md.ResourceMetrics().AppendEmpty() // Add an empty ResourceMetrics
	rm := md.ResourceMetrics().AppendEmpty()

	rm.Resource().Attributes().InsertString("labelB", "valueB")
	rm.Resource().Attributes().InsertString("labelA", "valueA")
	rm.Resource().Attributes().InsertString("a", "b")
	ilms := rm.InstrumentationLibraryMetrics()
	ilms.AppendEmpty() // Add an empty InstrumentationLibraryMetrics
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()

	badNameMetric := metrics.AppendEmpty()
	badNameMetric.SetName("")

	noneMetric := metrics.AppendEmpty()
	noneMetric.SetName("none")

	intGaugeMetric := metrics.AppendEmpty()
	intGaugeMetric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.IntGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	intGaugeDataPoint.SetValue(10)
	intGaugeDataPoint.SetTimestamp(pdata.Timestamp(100_000_000))

	doubleGaugeMetric := metrics.AppendEmpty()
	doubleGaugeMetric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	doubleGaugeMetric.SetName("double_gauge")
	doubleGauge := doubleGaugeMetric.DoubleGauge()
	doubleGaugeDataPoints := doubleGauge.DataPoints()
	doubleGaugeDataPoint := doubleGaugeDataPoints.AppendEmpty()
	doubleGaugeDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	doubleGaugeDataPoint.SetValue(10.1)
	doubleGaugeDataPoint.SetTimestamp(pdata.Timestamp(100_000_000))

	intSumMetric := metrics.AppendEmpty()
	intSumMetric.SetDataType(pdata.MetricDataTypeIntSum)
	intSumMetric.SetName("int_sum")
	intSum := intSumMetric.IntSum()
	intSumDataPoints := intSum.DataPoints()
	intSumDataPoint := intSumDataPoints.AppendEmpty()
	intSumDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	intSumDataPoint.SetValue(11)
	intSumDataPoint.SetTimestamp(pdata.Timestamp(100_000_000))

	doubleSumMetric := metrics.AppendEmpty()
	doubleSumMetric.SetDataType(pdata.MetricDataTypeDoubleSum)
	doubleSumMetric.SetName("double_sum")
	doubleSum := doubleSumMetric.DoubleSum()
	doubleSumDataPoints := doubleSum.DataPoints()
	doubleSumDataPoint := doubleSumDataPoints.AppendEmpty()
	doubleSumDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	doubleSumDataPoint.SetValue(10.1)
	doubleSumDataPoint.SetTimestamp(pdata.Timestamp(100_000_000))

	intHistogramMetric := metrics.AppendEmpty()
	intHistogramMetric.SetDataType(pdata.MetricDataTypeIntHistogram)
	intHistogramMetric.SetName("double_histogram")
	intHistogram := intHistogramMetric.IntHistogram()
	intHistogramDataPoints := intHistogram.DataPoints()
	intHistogramDataPoint := intHistogramDataPoints.AppendEmpty()
	intHistogramDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	intHistogramDataPoint.SetCount(2)
	intHistogramDataPoint.SetSum(19)
	intHistogramDataPoint.SetTimestamp(pdata.Timestamp(100_000_000))
	intHistogramDataPoint.SetBucketCounts([]uint64{1, 2, 3})
	intHistogramDataPoint.SetExplicitBounds([]float64{1, 2})

	doubleHistogramMetric := metrics.AppendEmpty()
	doubleHistogramMetric.SetDataType(pdata.MetricDataTypeHistogram)
	doubleHistogramMetric.SetName("double_$histogram")
	doubleHistogram := doubleHistogramMetric.Histogram()
	doubleHistogramDataPoints := doubleHistogram.DataPoints()
	doubleHistogramDataPoint := doubleHistogramDataPoints.AppendEmpty()
	doubleHistogramDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	doubleHistogramDataPoint.SetCount(2)
	doubleHistogramDataPoint.SetSum(10.1)
	doubleHistogramDataPoint.SetTimestamp(pdata.Timestamp(100_000_000))
	doubleHistogramDataPoint.SetBucketCounts([]uint64{1, 2, 3})
	doubleHistogramDataPoint.SetExplicitBounds([]float64{1, 2})

	doubleSummaryMetric := metrics.AppendEmpty()
	doubleSummaryMetric.SetDataType(pdata.MetricDataTypeSummary)
	doubleSummaryMetric.SetName("double-summary")
	doubleSummary := doubleSummaryMetric.Summary()
	doubleSummaryDataPoints := doubleSummary.DataPoints()
	doubleSummaryDataPoint := doubleSummaryDataPoints.AppendEmpty()
	doubleSummaryDataPoint.SetCount(2)
	doubleSummaryDataPoint.SetSum(10.1)
	doubleSummaryDataPoint.SetTimestamp(pdata.Timestamp(100_000_000))
	doubleSummaryDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	quantileVal := doubleSummaryDataPoint.QuantileValues().AppendEmpty()
	quantileVal.SetValue(10.2)
	quantileVal.SetQuantile(0.9)
	quantileVal2 := doubleSummaryDataPoint.QuantileValues().AppendEmpty()
	quantileVal2.SetValue(10.5)
	quantileVal2.SetQuantile(0.95)

	gotLogs := metricsDataToLogServiceData(logger, md)
	gotLogPairs := make([][]logKeyValuePair, 0, len(gotLogs))

	for _, log := range gotLogs {
		pairs := make([]logKeyValuePair, 0, len(log.Contents))
		for _, content := range log.Contents {
			pairs = append(pairs, logKeyValuePair{
				Key:   content.GetKey(),
				Value: content.GetValue(),
			})
		}
		gotLogPairs = append(gotLogPairs, pairs)

	}

	wantLogs := make([][]logKeyValuePair, 0, len(gotLogs))

	if err := loadFromJSON("./testdata/logservice_metric_data.json", &wantLogs); err != nil {
		t.Errorf("Failed load log key value pairs from file, error: %v", err)
		return
	}
	assert.Equal(t, len(wantLogs), len(gotLogs))
	for j := 0; j < len(gotLogs); j++ {
		sort.Sort(logKeyValuePairs(gotLogPairs[j]))
		sort.Sort(logKeyValuePairs(wantLogs[j]))
		if !reflect.DeepEqual(gotLogPairs[j], wantLogs[j]) {
			t.Errorf("Unsuccessful conversion \nGot:\n\t%v\nWant:\n\t%v", gotLogPairs, wantLogs)
		}
	}
}

func TestMetricCornerCases(t *testing.T) {
	assert.Equal(t, min(1, 2), 1)
	assert.Equal(t, min(2, 1), 1)
	assert.Equal(t, min(1, 1), 1)
	var label KeyValues
	label.Append("a", "b")
	assert.Equal(t, label.String(), "a#$#b")
}

func TestMetricLabelSanitize(t *testing.T) {
	var label KeyValues
	label.Append("_test", "key_test")
	label.Append("0test", "key_0test")
	label.Append("test_normal", "test_normal")
	label.Append("0test", "key_0test")
	assert.Equal(t, label.String(), "key_test#$#key_test|key_0test#$#key_0test|test_normal#$#test_normal|key_0test#$#key_0test")
	label.Sort()
	assert.Equal(t, label.String(), "key_0test#$#key_0test|key_0test#$#key_0test|key_test#$#key_test|test_normal#$#test_normal")
}
