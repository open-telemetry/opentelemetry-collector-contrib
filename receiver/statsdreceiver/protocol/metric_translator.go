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
	"go.opentelemetry.io/collector/consumer/pdata"
)

func buildCounterMetric(parsedMetric statsDMetric, timeNow time.Time) pdata.InstrumentationLibraryMetrics {
	ilm := pdata.NewInstrumentationLibraryMetrics()
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}
	nm.SetDataType(pdata.MetricDataTypeIntSum)
	nm.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	nm.IntSum().SetIsMonotonic(true)

	dp := nm.IntSum().DataPoints().AppendEmpty()
	dp.SetValue(parsedMetric.intvalue)
	dp.SetTimestamp(pdata.TimestampFromTime(timeNow))
	for i, key := range parsedMetric.labelKeys {
		dp.LabelsMap().Insert(key, parsedMetric.labelValues[i])
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
	nm.SetDataType(pdata.MetricDataTypeDoubleGauge)
	dp := nm.DoubleGauge().DataPoints().AppendEmpty()
	dp.SetValue(parsedMetric.floatvalue)
	dp.SetTimestamp(pdata.TimestampFromTime(timeNow))
	for i, key := range parsedMetric.labelKeys {
		dp.LabelsMap().Insert(key, parsedMetric.labelValues[i])
	}

	return ilm
}

func buildSummaryMetric(summaryMetric summaryMetric) pdata.InstrumentationLibraryMetrics {
	dp := pdata.NewSummaryDataPoint()
	dp.SetCount(uint64(len(summaryMetric.summaryPoints)))
	sum, _ := stats.Sum(summaryMetric.summaryPoints)
	dp.SetSum(sum)
	dp.SetTimestamp(pdata.TimestampFromTime(summaryMetric.timeNow))

	quantile := []float64{0, 10, 50, 90, 95, 100}
	for _, v := range quantile {
		eachQuantile := pdata.NewValueAtQuantile()
		eachQuantile.SetQuantile(v)
		eachQuantileValue, _ := stats.PercentileNearestRank(summaryMetric.summaryPoints, v)
		eachQuantile.SetValue(eachQuantileValue)
		dp.QuantileValues().Append(eachQuantile)
	}

	nm := pdata.NewMetric()
	nm.SetName(summaryMetric.name)
	nm.SetDataType(pdata.MetricDataTypeSummary)
	nm.Summary().DataPoints().Append(dp)

	ilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.Metrics().Append(nm)

	return ilm

}
