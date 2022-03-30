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

package golden

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestWriteMetrics(t *testing.T) {
	metricslice := testMetrics()
	metrics := pdata.NewMetrics()
	metricslice.CopyTo(metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())

	tempDir := filepath.Join(t.TempDir(), "metrics.json")
	WriteMetrics(tempDir, metrics)

	actualBytes, err := ioutil.ReadFile(tempDir)
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "roundtrip", "expected.json")
	expectedBytes, err := ioutil.ReadFile(expectedFile)
	require.NoError(t, err)

	require.Equal(t, expectedBytes, actualBytes)
}

func TestReadMetrics(t *testing.T) {
	metricslice := testMetrics()
	expectedMetrics := pdata.NewMetrics()
	metricslice.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())

	expectedFile := filepath.Join("testdata", "roundtrip", "expected.json")
	actualMetrics, err := ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, actualMetrics)
}

func TestRoundTrip(t *testing.T) {
	metricslice := testMetrics()
	expectedMetrics := pdata.NewMetrics()
	metricslice.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())

	tempDir := filepath.Join(t.TempDir(), "metrics.json")
	err := WriteMetrics(tempDir, expectedMetrics)
	require.NoError(t, err)

	actualMetrics, err := ReadMetrics(tempDir)
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, actualMetrics)
}

func testMetrics() pdata.MetricSlice {
	slice := pdata.NewMetricSlice()

	// Gauge with two double dps
	metric := slice.AppendEmpty()
	initGauge(metric, "test gauge multi", "multi gauge", "1")
	dps := metric.Gauge().DataPoints()

	dp := dps.AppendEmpty()
	attributes := pdata.NewMap()
	attributes.Insert("testKey1", pdata.NewValueString("teststringvalue1"))
	attributes.Insert("testKey2", pdata.NewValueString("testvalue1"))
	setDPDoubleVal(dp, 2, attributes, time.Time{})

	dp = dps.AppendEmpty()
	attributes = pdata.NewMap()
	attributes.Insert("testKey1", pdata.NewValueString("teststringvalue2"))
	attributes.Insert("testKey2", pdata.NewValueString("testvalue2"))
	setDPDoubleVal(dp, 2, attributes, time.Time{})

	// Gauge with one int dp
	metric = slice.AppendEmpty()
	initGauge(metric, "test gauge single", "single gauge", "By")
	dps = metric.Gauge().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pdata.NewMap()
	attributes.Insert("testKey2", pdata.NewValueString("teststringvalue2"))
	setDPIntVal(dp, 2, attributes, time.Time{})

	// Delta Sum with two int dps
	metric = slice.AppendEmpty()
	initSum(metric, "test delta sum multi", "multi sum", "s", pdata.MetricAggregationTemporalityDelta, false)
	dps = metric.Sum().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pdata.NewMap()
	attributes.Insert("testKey2", pdata.NewValueString("teststringvalue2"))
	setDPIntVal(dp, 2, attributes, time.Time{})

	dp = dps.AppendEmpty()
	attributes = pdata.NewMap()
	attributes.Insert("testKey2", pdata.NewValueString("teststringvalue2"))
	setDPIntVal(dp, 2, attributes, time.Time{})

	// Cumulative Sum with one double dp
	metric = slice.AppendEmpty()
	initSum(metric, "test cumulative sum single", "single sum", "1/s", pdata.MetricAggregationTemporalityCumulative, true)
	dps = metric.Sum().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pdata.NewMap()
	setDPDoubleVal(dp, 2, attributes, time.Date(1997, 07, 27, 1, 1, 1, 1, &time.Location{}))
	return slice
}

func setDPDoubleVal(dp pdata.NumberDataPoint, value float64, attributes pdata.Map, timeStamp time.Time) {
	dp.SetDoubleVal(value)
	dp.SetTimestamp(pdata.NewTimestampFromTime(timeStamp))
	attributes.CopyTo(dp.Attributes())
}

func setDPIntVal(dp pdata.NumberDataPoint, value int64, attributes pdata.Map, timeStamp time.Time) {
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
