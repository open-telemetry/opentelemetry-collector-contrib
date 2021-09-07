// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/dotnet"
)

func TestMeanMetricToPdata(t *testing.T) {
	jsonFile := 0
	expectedName := "dotnet.cpu-usage"
	expectedUnits := "%"
	pdm := testMetricConversion(t, jsonFile, expectedName, expectedUnits)
	pts := pdm.Gauge().DataPoints()
	assert.Equal(t, 1, pts.Len())
	pt := pts.At(0)
	assert.EqualValues(t, 0, pt.StartTimestamp())
	assert.Equal(t, pdata.NewTimestampFromTime(time.Unix(111, 0)), pt.Timestamp())
	assert.Equal(t, 0.5, pt.DoubleVal())
}

func TestSumMetricToPdata(t *testing.T) {
	jsonFile := 16
	expectedName := "dotnet.alloc-rate"
	expectedUnits := "By"
	pdm := testMetricConversion(t, jsonFile, expectedName, expectedUnits)
	sum := pdm.Sum()
	assert.False(t, sum.IsMonotonic())
	pts := sum.DataPoints()
	assert.Equal(t, 1, pts.Len())
	pt := pts.At(0)
	assert.Equal(t, pdata.NewTimestampFromTime(time.Unix(42, 0)), pt.StartTimestamp())
	assert.Equal(t, pdata.NewTimestampFromTime(time.Unix(111, 0)), pt.Timestamp())
	assert.Equal(t, 262672.0, pt.DoubleVal())
}

func testMetricConversion(t *testing.T, metricFile int, expectedName string, expectedUnits string) pdata.Metric {
	rm := readTestdataMetric(metricFile)
	pdms := rawMetricsToPdata([]dotnet.Metric{rm}, time.Unix(42, 0), time.Unix(111, 0))
	rms := pdms.ResourceMetrics()
	assert.Equal(t, 1, rms.Len())
	ilms := rms.At(0).InstrumentationLibraryMetrics()
	assert.Equal(t, 1, ilms.Len())
	ms := ilms.At(0).Metrics()
	assert.Equal(t, 1, ms.Len())
	pdm := ms.At(0)
	assert.Equal(t, expectedName, pdm.Name())
	assert.Equal(t, expectedUnits, pdm.Unit())
	return pdm
}

func readTestdataMetric(i int) dotnet.Metric {
	bytes, err := ioutil.ReadFile(testdataMetricFname(i))
	if err != nil {
		panic(err)
	}
	m := dotnet.Metric{}
	err = json.Unmarshal(bytes, &m)
	if err != nil {
		panic(err)
	}
	return m
}

func testdataMetricFname(i int) string {
	return fmt.Sprintf("../testdata/metric.%d.json", i)
}
