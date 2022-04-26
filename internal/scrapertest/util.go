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

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func metricsByName(metricSlice pmetric.MetricSlice) map[string]pmetric.Metric {
	byName := make(map[string]pmetric.Metric, metricSlice.Len())
	for i := 0; i < metricSlice.Len(); i++ {
		a := metricSlice.At(i)
		byName[a.Name()] = a
	}
	return byName
}

func getDataPointSlice(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	var dataPointSlice pmetric.NumberDataPointSlice
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		dataPointSlice = metric.Gauge().DataPoints()
	case pmetric.MetricDataTypeSum:
		dataPointSlice = metric.Sum().DataPoints()
	default:
		panic(fmt.Sprintf("data type not supported: %s", metric.DataType()))
	}
	return dataPointSlice
}

func sortInstrumentationLibrary(a, b pmetric.ScopeMetrics) bool {
	if a.SchemaUrl() < b.SchemaUrl() {
		return true
	}
	if a.Scope().Name() < b.Scope().Name() {
		return true
	}
	if a.Scope().Version() < b.Scope().Version() {
		return true
	}
	return false
}
