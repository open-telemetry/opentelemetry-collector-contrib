// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scrapertest

import (
	"time"

	"go.opentelemetry.io/collector/model/pdata"
)

func baseTestMetrics() pdata.MetricSlice {
	slice := pdata.NewMetricSlice()

	// set Gauge with two double dps
	metric := slice.AppendEmpty()
	initGauge(metric, "test gauge multi", "multi gauge", "1")
	dps := metric.Gauge().DataPoints()

	dp := dps.AppendEmpty()
	attributes := pdata.NewAttributeMap()
	attributes.Insert("testKey1", pdata.NewAttributeValueString("teststringvalue1"))
	attributes.Insert("testKey2", pdata.NewAttributeValueString("testvalue1"))
	setDPDoubleVal(dp, 2, attributes, time.Time{})

	dp = dps.AppendEmpty()
	attributes = pdata.NewAttributeMap()
	attributes.Insert("testKey1", pdata.NewAttributeValueString("teststringvalue2"))
	attributes.Insert("testKey2", pdata.NewAttributeValueString("testvalue2"))
	setDPDoubleVal(dp, 2, attributes, time.Time{})

	// set Gauge with one int dp
	metric = slice.AppendEmpty()
	initGauge(metric, "test gauge single", "single gauge", "By")
	dps = metric.Gauge().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pdata.NewAttributeMap()
	attributes.Insert("testKey2", pdata.NewAttributeValueString("teststringvalue2"))
	setDPIntVal(dp, 2, attributes, time.Time{})

	// set Delta Sum with two int dps
	metric = slice.AppendEmpty()
	initSum(metric, "test delta sum multi", "multi sum", "s", pdata.MetricAggregationTemporalityDelta, false)
	dps = metric.Sum().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pdata.NewAttributeMap()
	attributes.Insert("testKey2", pdata.NewAttributeValueString("teststringvalue2"))
	setDPIntVal(dp, 2, attributes, time.Time{})

	dp = dps.AppendEmpty()
	attributes = pdata.NewAttributeMap()
	attributes.Insert("testKey2", pdata.NewAttributeValueString("teststringvalue2"))
	setDPIntVal(dp, 2, attributes, time.Time{})

	// set Cumulative Sum with one double dp
	metric = slice.AppendEmpty()
	initSum(metric, "test cumulative sum single", "single sum", "1/s", pdata.MetricAggregationTemporalityCumulative, true)
	dps = metric.Sum().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pdata.NewAttributeMap()
	setDPDoubleVal(dp, 2, attributes, time.Date(1997, 07, 27, 1, 1, 1, 1, &time.Location{}))
	return slice
}

func setDPDoubleVal(dp pdata.NumberDataPoint, value float64, attributes pdata.AttributeMap, timeStamp time.Time) {
	dp.SetDoubleVal(value)
	dp.SetTimestamp(pdata.NewTimestampFromTime(timeStamp))
	attributes.CopyTo(dp.Attributes())
}

func setDPIntVal(dp pdata.NumberDataPoint, value int64, attributes pdata.AttributeMap, timeStamp time.Time) {
	dp.SetIntVal(value)
	dp.SetTimestamp(pdata.NewTimestampFromTime(timeStamp))
	attributes.CopyTo(dp.Attributes())
}

func initGauge(metric pdata.Metric, name, desc, unit string) {
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetDescription(desc)
	metric.SetName(name)
	metric.SetUnit(unit)
}

func initSum(metric pdata.Metric, name, desc, unit string, aggr pdata.MetricAggregationTemporality, isMonotonic bool) {
	metric.SetDataType(pdata.MetricDataTypeSum)
	metric.Sum().SetIsMonotonic(isMonotonic)
	metric.Sum().SetAggregationTemporality(aggr)
	metric.SetDescription(desc)
	metric.SetName(name)
	metric.SetUnit(unit)
}
