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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestMetricDataToLogService(t *testing.T) {
	logger := zap.NewNop()

	md := pdata.NewMetrics()
	md.ResourceMetrics().Resize(2)
	rm := md.ResourceMetrics().At(0)

	rm.Resource().Attributes().InsertString("labelB", "valueB")
	rm.Resource().Attributes().InsertString("labelA", "valueA")
	rm.Resource().Attributes().InsertString("a", "b")
	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Resize(2)
	ilm := ilms.At(0)

	metrics := ilm.Metrics()
	metrics.Resize(10)

	badNameMetric := metrics.At(0)
	badNameMetric.SetName("")

	noneMetric := metrics.At(1)
	noneMetric.SetName("none")

	intGaugeMetric := metrics.At(2)
	intGaugeMetric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.IntGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoints.Resize(1)
	intGaugeDataPoint := intGaugeDataPoints.At(0)
	intGaugeDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	intGaugeDataPoint.SetValue(10)
	intGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	doubleGaugeMetric := metrics.At(3)
	doubleGaugeMetric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	doubleGaugeMetric.SetName("double_gauge")
	doubleGauge := doubleGaugeMetric.DoubleGauge()
	doubleGaugeDataPoints := doubleGauge.DataPoints()
	doubleGaugeDataPoints.Resize(1)
	doubleGaugeDataPoint := doubleGaugeDataPoints.At(0)
	doubleGaugeDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	doubleGaugeDataPoint.SetValue(10.1)
	doubleGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	intSumMetric := metrics.At(4)
	intSumMetric.SetDataType(pdata.MetricDataTypeIntSum)
	intSumMetric.SetName("int_sum")
	intSum := intSumMetric.IntSum()
	intSumDataPoints := intSum.DataPoints()
	intSumDataPoints.Resize(1)
	intSumDataPoint := intSumDataPoints.At(0)
	intSumDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	intSumDataPoint.SetValue(11)
	intSumDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	doubleSumMetric := metrics.At(5)
	doubleSumMetric.SetDataType(pdata.MetricDataTypeDoubleSum)
	doubleSumMetric.SetName("double_sum")
	doubleSum := doubleSumMetric.DoubleSum()
	doubleSumDataPoints := doubleSum.DataPoints()
	doubleSumDataPoints.Resize(1)
	doubleSumDataPoint := doubleSumDataPoints.At(0)
	doubleSumDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	doubleSumDataPoint.SetValue(10.1)
	doubleSumDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	intHistogramMetric := metrics.At(6)
	intHistogramMetric.SetDataType(pdata.MetricDataTypeIntHistogram)
	intHistogramMetric.SetName("double_histogram")
	intHistogram := intHistogramMetric.IntHistogram()
	intHistogramDataPoints := intHistogram.DataPoints()
	intHistogramDataPoints.Resize(1)
	intHistogramDataPoint := intHistogramDataPoints.At(0)
	intHistogramDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	intHistogramDataPoint.SetCount(2)
	intHistogramDataPoint.SetSum(19)
	intHistogramDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))
	intHistogramDataPoint.SetBucketCounts([]uint64{1, 2, 3})
	intHistogramDataPoint.SetExplicitBounds([]float64{1, 2})

	doubleHistogramMetric := metrics.At(7)
	doubleHistogramMetric.SetDataType(pdata.MetricDataTypeDoubleHistogram)
	doubleHistogramMetric.SetName("double_$histogram")
	doubleHistogram := doubleHistogramMetric.DoubleHistogram()
	doubleHistogramDataPoints := doubleHistogram.DataPoints()
	doubleHistogramDataPoints.Resize(1)
	doubleHistogramDataPoint := doubleHistogramDataPoints.At(0)
	doubleHistogramDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	doubleHistogramDataPoint.SetCount(2)
	doubleHistogramDataPoint.SetSum(10.1)
	doubleHistogramDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))
	doubleHistogramDataPoint.SetBucketCounts([]uint64{1, 2, 3})
	doubleHistogramDataPoint.SetExplicitBounds([]float64{1, 2})

	doubleSummaryMetric := metrics.At(8)
	doubleSummaryMetric.SetDataType(pdata.MetricDataTypeDoubleSummary)
	doubleSummaryMetric.SetName("double-summary")
	doubleSummary := doubleSummaryMetric.DoubleSummary()
	doubleSummaryDataPoints := doubleSummary.DataPoints()
	doubleSummaryDataPoints.Resize(1)
	doubleSummaryDataPoint := doubleSummaryDataPoints.At(0)
	doubleSummaryDataPoint.SetCount(2)
	doubleSummaryDataPoint.SetSum(10.1)
	doubleSummaryDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))
	doubleSummaryDataPoint.LabelsMap().Insert("innerLabel", "innerValue")
	quantileVal := pdata.NewValueAtQuantile()
	quantileVal.SetValue(10.2)
	quantileVal.SetQuantile(0.9)
	quantileVal2 := pdata.NewValueAtQuantile()
	quantileVal2.SetValue(10.5)
	quantileVal2.SetQuantile(0.95)
	doubleSummaryDataPoint.QuantileValues().Append(quantileVal)
	doubleSummaryDataPoint.QuantileValues().Append(quantileVal2)

	gotLogs, gotNumDroppedTimeSeries := metricsDataToLogServiceData(logger, md)
	assert.Equal(t, gotNumDroppedTimeSeries, 0)
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
	label.AppendMap(map[string]string{
		"a": "b",
	})
	assert.Equal(t, label.String(), "a#$#b")
}
