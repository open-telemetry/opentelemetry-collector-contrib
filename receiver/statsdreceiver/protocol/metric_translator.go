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
	"time"

	"github.com/montanaflynn/stats"
	"go.opentelemetry.io/collector/model/pdata"
)

func buildCounterMetric(parsedMetric statsDMetric, isMonotonicCounter bool, timeNow time.Time) pdata.InstrumentationLibraryMetrics {
	ilm := pdata.NewInstrumentationLibraryMetrics()
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}
	nm.SetDataType(pdata.MetricDataTypeSum)

	nm.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	if isMonotonicCounter {
		nm.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	}

	nm.Sum().SetIsMonotonic(true)

	dp := nm.Sum().DataPoints().AppendEmpty()
	dp.SetIntVal(parsedMetric.counterValue())
	dp.SetTimestamp(pdata.NewTimestampFromTime(timeNow))
	for i, key := range parsedMetric.labelKeys {
		dp.Attributes().InsertString(key, parsedMetric.labelValues[i])
	}

	return ilm
}

func buildGaugeMetric(parsedMetric statsDMetric, timeNow time.Time) pdata.InstrumentationLibraryMetrics {
	ilm := pdata.NewInstrumentationLibraryMetrics()
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}
	nm.SetDataType(pdata.MetricDataTypeGauge)
	dp := nm.Gauge().DataPoints().AppendEmpty()
	dp.SetDoubleVal(parsedMetric.gaugeValue())
	dp.SetTimestamp(pdata.NewTimestampFromTime(timeNow))
	for i, key := range parsedMetric.labelKeys {
		dp.Attributes().InsertString(key, parsedMetric.labelValues[i])
	}

	return ilm
}

func buildSummaryMetric(summaryMetric summaryMetric) pdata.InstrumentationLibraryMetrics {
	ilm := pdata.NewInstrumentationLibraryMetrics()
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(summaryMetric.name)
	nm.SetDataType(pdata.MetricDataTypeSummary)

	dp := nm.Summary().DataPoints().AppendEmpty()
	dp.SetCount(uint64(len(summaryMetric.summaryPoints)))
	sum, _ := stats.Sum(summaryMetric.summaryPoints)
	dp.SetSum(sum)
	dp.SetTimestamp(pdata.NewTimestampFromTime(summaryMetric.timeNow))
	for i, key := range summaryMetric.labelKeys {
		dp.Attributes().InsertString(key, summaryMetric.labelValues[i])
	}

	quantile := []float64{0, 10, 50, 90, 95, 100}
	for _, v := range quantile {
		eachQuantile := dp.QuantileValues().AppendEmpty()
		eachQuantile.SetQuantile(v / 100)
		eachQuantileValue, _ := stats.PercentileNearestRank(summaryMetric.summaryPoints, v)
		eachQuantile.SetValue(eachQuantileValue)
	}
	return ilm

}

func (s statsDMetric) counterValue() int64 {
	x := s.asFloat
	// Note statds counters are always represented as integers.
	// There is no statsd specification that says what should or
	// shouldn't be done here.  Rounding may occur for sample
	// rates that are not integer reciprocals.  Recommendation:
	// use integer reciprocal sampling rates.
	if 0 < s.sampleRate && s.sampleRate < 1 {
		x = x / s.sampleRate
	}
	return int64(x)
}

func (s statsDMetric) gaugeValue() float64 {
	// sampleRate does not have effect for gauge points.
	return s.asFloat
}

func (s statsDMetric) summaryValue() float64 {
	// TODO: This method returns an incorrect result.  The
	// sampleRate is meant to apply to the count of observations
	// recorded by the histogram/timer, not to scale the value
	// observed.  This should return gaugeValue(), and the
	// consumer of this value should track the effective count of
	// each item to compute percentiles. See #5252.
	x := s.asFloat
	if 0 < s.sampleRate && s.sampleRate < 1 {
		x = x / s.sampleRate
	}
	return x
}
