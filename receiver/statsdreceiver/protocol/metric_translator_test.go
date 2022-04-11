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
	"go.opentelemetry.io/otel/attribute"
)

func TestBuildCounterMetric(t *testing.T) {
	timeNow := time.Now()
	lastUpdateInterval := timeNow.Add(-1 * time.Minute)
	metricDescription := statsDMetricDescription{
		name:  "testCounter",
		attrs: attribute.NewSet(attribute.String("mykey", "myvalue")),
	}

	parsedMetric := statsDMetric{
		description: metricDescription,
		asFloat:     32,
		unit:        "meter",
	}
	isMonotonicCounter := false
	metric := buildCounterMetric(parsedMetric, isMonotonicCounter, timeNow, lastUpdateInterval)
	expectedMetrics := pdata.NewScopeMetrics()
	expectedMetric := expectedMetrics.Metrics().AppendEmpty()
	expectedMetric.SetName("testCounter")
	expectedMetric.SetUnit("meter")
	expectedMetric.SetDataType(pdata.MetricDataTypeSum)
	expectedMetric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	expectedMetric.Sum().SetIsMonotonic(isMonotonicCounter)
	dp := expectedMetric.Sum().DataPoints().AppendEmpty()
	dp.SetIntVal(32)
	dp.SetStartTimestamp(pdata.NewTimestampFromTime(lastUpdateInterval))
	dp.SetTimestamp(pdata.NewTimestampFromTime(timeNow))
	dp.Attributes().InsertString("mykey", "myvalue")
	assert.Equal(t, metric, expectedMetrics)
}

func TestBuildGaugeMetric(t *testing.T) {
	timeNow := time.Now()
	metricDescription := statsDMetricDescription{
		name: "testGauge",
		attrs: attribute.NewSet(
			attribute.String("mykey", "myvalue"),
			attribute.String("mykey2", "myvalue2"),
		),
	}
	parsedMetric := statsDMetric{
		description: metricDescription,
		asFloat:     32.3,
		unit:        "meter",
	}
	metric := buildGaugeMetric(parsedMetric, timeNow)
	expectedMetrics := pdata.NewScopeMetrics()
	expectedMetric := expectedMetrics.Metrics().AppendEmpty()
	expectedMetric.SetName("testGauge")
	expectedMetric.SetUnit("meter")
	expectedMetric.SetDataType(pdata.MetricDataTypeGauge)
	dp := expectedMetric.Gauge().DataPoints().AppendEmpty()
	dp.SetDoubleVal(32.3)
	dp.SetTimestamp(pdata.NewTimestampFromTime(timeNow))
	dp.Attributes().InsertString("mykey", "myvalue")
	dp.Attributes().InsertString("mykey2", "myvalue2")
	assert.Equal(t, metric, expectedMetrics)
}

func TestBuildSummaryMetricUnsampled(t *testing.T) {
	timeNow := time.Now()

	unsampledMetric := summaryMetric{
		points:  []float64{1, 2, 4, 6, 5, 3},
		weights: []float64{1, 1, 1, 1, 1, 1},
	}

	attrs := attribute.NewSet(
		attribute.String("mykey", "myvalue"),
		attribute.String("mykey2", "myvalue2"),
	)

	desc := statsDMetricDescription{
		name:       "testSummary",
		metricType: HistogramType,
		attrs:      attrs,
	}

	metric := pdata.NewScopeMetrics()
	buildSummaryMetric(desc, unsampledMetric, timeNow.Add(-time.Minute), timeNow, statsDDefaultPercentiles, metric)

	expectedMetric := pdata.NewScopeMetrics()
	m := expectedMetric.Metrics().AppendEmpty()
	m.SetName("testSummary")
	m.SetDataType(pdata.MetricDataTypeSummary)
	dp := m.Summary().DataPoints().AppendEmpty()
	dp.SetSum(21)
	dp.SetCount(6)
	dp.SetStartTimestamp(pdata.NewTimestampFromTime(timeNow.Add(-time.Minute)))
	dp.SetTimestamp(pdata.NewTimestampFromTime(timeNow))
	for _, kv := range desc.attrs.ToSlice() {
		dp.Attributes().InsertString(string(kv.Key), kv.Value.AsString())
	}
	quantile := []float64{0, 10, 50, 90, 95, 100}
	value := []float64{1, 1, 3, 6, 6, 6}
	for int, v := range quantile {
		eachQuantile := dp.QuantileValues().AppendEmpty()
		eachQuantile.SetQuantile(v / 100)
		eachQuantileValue := value[int]
		eachQuantile.SetValue(eachQuantileValue)
	}

	assert.Equal(t, expectedMetric, metric)
}

func TestBuildSummaryMetricSampled(t *testing.T) {
	timeNow := time.Now()

	type testCase struct {
		points      []float64
		weights     []float64
		count       uint64
		sum         float64
		percentiles []float64
		values      []float64
	}

	for _, test := range []testCase{
		{
			points:      []float64{1, 2, 3},
			weights:     []float64{100, 1, 100},
			count:       201,
			sum:         402,
			percentiles: []float64{0, 1, 49, 50, 51, 99, 100},
			values:      []float64{1, 1, 1, 2, 3, 3, 3},
		},
		{
			points:      []float64{1, 2},
			weights:     []float64{99, 1},
			count:       100,
			sum:         101,
			percentiles: []float64{0, 98, 99, 100},
			values:      []float64{1, 1, 1, 2},
		},
		{
			points:      []float64{0, 1, 2, 3, 4, 5},
			weights:     []float64{1, 9, 40, 40, 5, 5},
			count:       100,
			sum:         254,
			percentiles: statsDDefaultPercentiles,
			values:      []float64{0, 1, 2, 3, 4, 5},
		},
	} {
		sampledMetric := summaryMetric{
			points:  test.points,
			weights: test.weights,
		}

		attrs := attribute.NewSet(
			attribute.String("mykey", "myvalue"),
			attribute.String("mykey2", "myvalue2"),
		)

		desc := statsDMetricDescription{
			name:       "testSummary",
			metricType: HistogramType,
			attrs:      attrs,
		}

		metric := pdata.NewScopeMetrics()
		buildSummaryMetric(desc, sampledMetric, timeNow.Add(-time.Minute), timeNow, test.percentiles, metric)

		expectedMetric := pdata.NewScopeMetrics()
		m := expectedMetric.Metrics().AppendEmpty()
		m.SetName("testSummary")
		m.SetDataType(pdata.MetricDataTypeSummary)
		dp := m.Summary().DataPoints().AppendEmpty()

		dp.SetSum(test.sum)
		dp.SetCount(test.count)

		dp.SetStartTimestamp(pdata.NewTimestampFromTime(timeNow.Add(-time.Minute)))
		dp.SetTimestamp(pdata.NewTimestampFromTime(timeNow))
		for _, kv := range desc.attrs.ToSlice() {
			dp.Attributes().InsertString(string(kv.Key), kv.Value.AsString())
		}
		for i := range test.percentiles {
			eachQuantile := dp.QuantileValues().AppendEmpty()
			eachQuantile.SetQuantile(test.percentiles[i] / 100)
			eachQuantile.SetValue(test.values[i])
		}

		assert.Equal(t, expectedMetric, metric)
	}
}
