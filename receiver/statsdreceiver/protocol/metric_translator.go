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

	"go.opentelemetry.io/collector/consumer/pdata"
)

func buildCounterMetric(parsedMetric statsDMetric, timeNow time.Time) pdata.InstrumentationLibraryMetrics {
	dp := pdata.NewIntDataPoint()
	dp.SetValue(parsedMetric.intvalue)
	dp.SetTimestamp(pdata.TimestampFromTime(timeNow))
	for i, key := range parsedMetric.labelKeys {
		dp.LabelsMap().Insert(key, parsedMetric.labelValues[i])
	}

	nm := pdata.NewMetric()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}
	nm.SetDataType(pdata.MetricDataTypeIntSum)
	nm.IntSum().DataPoints().Append(dp)
	nm.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	nm.IntSum().SetIsMonotonic(true)

	ilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.Metrics().Append(nm)

	return ilm
}

func buildGaugeMetric(parsedMetric statsDMetric, timeNow time.Time) pdata.InstrumentationLibraryMetrics {
	dp := pdata.NewDoubleDataPoint()
	dp.SetValue(parsedMetric.floatvalue)
	dp.SetTimestamp(pdata.TimestampFromTime(timeNow))
	for i, key := range parsedMetric.labelKeys {
		dp.LabelsMap().Insert(key, parsedMetric.labelValues[i])
	}

	nm := pdata.NewMetric()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}
	nm.SetDataType(pdata.MetricDataTypeDoubleGauge)
	nm.DoubleGauge().DataPoints().Append(dp)

	ilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.Metrics().Append(nm)

	return ilm
}
