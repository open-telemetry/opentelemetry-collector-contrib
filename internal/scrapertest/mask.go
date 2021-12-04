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

// IgnoreValues is a CompareOption that clears all values
func IgnoreValues() CompareOption {
	return ignoreValues{}
}

type ignoreValues struct{}

func (opt ignoreValues) apply(expected, actual pdata.MetricSlice) {
	maskMetricSliceValues(expected)
	maskMetricSliceValues(actual)
}

// maskMetricSliceValues sets all data point values to zero.
func maskMetricSliceValues(metrics pdata.MetricSlice) {
	for i := 0; i < metrics.Len(); i++ {
		maskMetricValues(metrics.At(i))
	}
}

// maskMetricValues sets all data point values to zero.
func maskMetricValues(metric pdata.Metric) {
	var dataPoints pdata.NumberDataPointSlice
	switch metric.DataType() {
	case pdata.MetricDataTypeGauge:
		dataPoints = metric.Gauge().DataPoints()
	case pdata.MetricDataTypeSum:
		dataPoints = metric.Sum().DataPoints()
	default:
		panic(fmt.Sprintf("data type not supported: %s", metric.DataType()))
	}
	maskDataPointSliceValues(dataPoints)
}

// maskDataPointSliceValues sets all data point values to zero.
func maskDataPointSliceValues(dataPoints pdata.NumberDataPointSlice) {
	for i := 0; i < dataPoints.Len(); i++ {
		dataPoint := dataPoints.At(i)
		dataPoint.SetIntVal(0)
		dataPoint.SetDoubleVal(0)
	}
}
