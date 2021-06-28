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

package protocol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestBuildCounterMetric(t *testing.T) {
	timeNow := time.Now()
	metricDescription := statsDMetricdescription{
		name: "testCounter",
	}
	parsedMetric := statsDMetric{
		description: metricDescription,
		intvalue:    32,
		unit:        "meter",
		labelKeys:   []string{"mykey"},
		labelValues: []string{"myvalue"},
	}
	metric := buildCounterMetric(parsedMetric, timeNow)
	expectedMetrics := pdata.NewInstrumentationLibraryMetrics()
	expectedMetric := expectedMetrics.Metrics().AppendEmpty()
	expectedMetric.SetName("testCounter")
	expectedMetric.SetUnit("meter")
	expectedMetric.SetDataType(pdata.MetricDataTypeIntSum)
	expectedMetric.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	expectedMetric.IntSum().SetIsMonotonic(true)
	dp := expectedMetric.IntSum().DataPoints().AppendEmpty()
	dp.SetValue(32)
	dp.SetTimestamp(pdata.TimestampFromTime(timeNow))
	dp.LabelsMap().Insert("mykey", "myvalue")
	assert.Equal(t, metric, expectedMetrics)
}

func TestBuildGaugeMetric(t *testing.T) {
	timeNow := time.Now()
	metricDescription := statsDMetricdescription{
		name: "testGauge",
	}
	parsedMetric := statsDMetric{
		description: metricDescription,
		floatvalue:  32.3,
		unit:        "meter",
		labelKeys:   []string{"mykey", "mykey2"},
		labelValues: []string{"myvalue", "myvalue2"},
	}
	metric := buildGaugeMetric(parsedMetric, timeNow)
	expectedMetrics := pdata.NewInstrumentationLibraryMetrics()
	expectedMetric := expectedMetrics.Metrics().AppendEmpty()
	expectedMetric.SetName("testGauge")
	expectedMetric.SetUnit("meter")
	expectedMetric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	dp := expectedMetric.DoubleGauge().DataPoints().AppendEmpty()
	dp.SetValue(32.3)
	dp.SetTimestamp(pdata.TimestampFromTime(timeNow))
	dp.LabelsMap().Insert("mykey", "myvalue")
	dp.LabelsMap().Insert("mykey2", "myvalue2")
	assert.Equal(t, metric, expectedMetrics)
}

func TestBuildSummaryMetric(t *testing.T) {
	timeNow := time.Now()

	oneSummaryMetric := summaryMetric{
		name:          "testSummary",
		summaryPoints: []float64{1, 2, 4, 6, 5, 3},
		labelKeys:     []string{"mykey", "mykey2"},
		labelValues:   []string{"myvalue", "myvalue2"},
		timeNow:       timeNow,
	}

	metric := buildSummaryMetric(oneSummaryMetric)
	expectedMetric := pdata.NewInstrumentationLibraryMetrics()
	expectedMetric.Metrics().Resize(1)
	expectedMetric.Metrics().At(0).SetName("testSummary")
	expectedMetric.Metrics().At(0).SetDataType(pdata.MetricDataTypeSummary)
	expectedMetric.Metrics().At(0).Summary().DataPoints().Resize(1)
	expectedMetric.Metrics().At(0).Summary().DataPoints().At(0).SetSum(21)
	expectedMetric.Metrics().At(0).Summary().DataPoints().At(0).SetCount(6)
	expectedMetric.Metrics().At(0).Summary().DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timeNow))
	for i, key := range oneSummaryMetric.labelKeys {
		expectedMetric.Metrics().At(0).Summary().DataPoints().At(0).LabelsMap().Insert(key, oneSummaryMetric.labelValues[i])
	}
	quantile := []float64{0, 10, 50, 90, 95, 100}
	value := []float64{1, 1, 3, 6, 6, 6}
	for int, v := range quantile {
		eachQuantile := expectedMetric.Metrics().At(0).Summary().DataPoints().At(0).QuantileValues().AppendEmpty()
		eachQuantile.SetQuantile(v)
		eachQuantileValue := value[int]
		eachQuantile.SetValue(eachQuantileValue)
	}

	assert.Equal(t, metric, expectedMetric)

}
