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

package scrapertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"

import (
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"
)

func metricsByName(metricSlice pdata.MetricSlice) map[string]pdata.Metric {
	byName := make(map[string]pdata.Metric, metricSlice.Len())
	for i := 0; i < metricSlice.Len(); i++ {
		a := metricSlice.At(i)
		byName[a.Name()] = a
	}
	return byName
}

func getDataPointSlice(metric pdata.Metric) pdata.NumberDataPointSlice {
	var dataPointSlice pdata.NumberDataPointSlice
	switch metric.DataType() {
	case pdata.MetricDataTypeGauge:
		dataPointSlice = metric.Gauge().DataPoints()
	case pdata.MetricDataTypeSum:
		dataPointSlice = metric.Sum().DataPoints()
	default:
		panic(fmt.Sprintf("data type not supported: %s", metric.DataType()))
	}
	return dataPointSlice
}

func sortInstrumentationLibrary(a, b pdata.InstrumentationLibraryMetrics) bool {
	if a.SchemaUrl() < b.SchemaUrl() {
		return true
	}
	if a.InstrumentationLibrary().Name() < b.InstrumentationLibrary().Name() {
		return true
	}
	if a.InstrumentationLibrary().Version() < b.InstrumentationLibrary().Version() {
		return true
	}
	return false
}
