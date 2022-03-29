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

// IgnoreMetricValues is a CompareOption that clears all values
func IgnoreMetricValues() CompareOption {
	return ignoreMetricValues{}
}

type ignoreMetricValues struct{}

func (opt ignoreMetricValues) apply(expected, actual pdata.Metrics) {
	maskMetricValues(expected)
	maskMetricValues(actual)
}

func maskMetricValues(metrics pdata.Metrics) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			maskMetricSliceValues(ilms.At(j).Metrics())
		}
	}
}

// maskMetricSliceValues sets all data point values to zero.
func maskMetricSliceValues(metrics pdata.MetricSlice) {
	for i := 0; i < metrics.Len(); i++ {
		maskDataPointSliceValues(getDataPointSlice(metrics.At(i)))
	}
}

// maskDataPointSliceValues sets all data point values to zero.
func maskDataPointSliceValues(dataPoints pdata.NumberDataPointSlice) {
	for i := 0; i < dataPoints.Len(); i++ {
		dataPoint := dataPoints.At(i)
		dataPoint.SetIntVal(0)
		dataPoint.SetDoubleVal(0)
	}
}

// IgnoreMetricAttributeValue is a CompareOption that clears all values
func IgnoreMetricAttributeValue(attributeName string, metricNames ...string) CompareOption {
	return ignoreMetricAttributeValue{
		attributeName: attributeName,
		metricNames:   metricNames,
	}
}

type ignoreMetricAttributeValue struct {
	attributeName string
	metricNames   []string
}

func (opt ignoreMetricAttributeValue) apply(expected, actual pdata.Metrics) {
	maskMetricAttributeValue(expected, opt)
	maskMetricAttributeValue(actual, opt)
}

func maskMetricAttributeValue(metrics pdata.Metrics, opt ignoreMetricAttributeValue) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			maskMetricSliceAttributeValues(ilms.At(j).Metrics(), opt.attributeName, opt.metricNames...)
		}
	}
}

// maskMetricSliceAttributeValues sets the value of the specified attribute to
// the zero value associated with the attribute data type.
// If metric names are specified, only the data points within those metrics will be masked.
// Otherwise, all data points with the attribute will be masked.
func maskMetricSliceAttributeValues(metrics pdata.MetricSlice, attributeName string, metricNames ...string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}

	for i := 0; i < metrics.Len(); i++ {
		if len(metricNames) == 0 || metricNameSet[metrics.At(i).Name()] {
			dps := getDataPointSlice(metrics.At(i))
			maskDataPointSliceAttributeValues(dps, attributeName)

			// If attribute values are ignored, some data points may become
			// indistinguishable from each other, but sorting by value allows
			// for a reasonably thorough comparison and a deterministic outcome.
			dps.Sort(func(a, b pdata.NumberDataPoint) bool {
				if a.IntVal() < b.IntVal() {
					return true
				}
				if a.DoubleVal() < b.DoubleVal() {
					return true
				}
				return false
			})
		}
	}
}

// maskDataPointSliceAttributeValues sets the value of the specified attribute to
// the zero value associated with the attribute data type.
func maskDataPointSliceAttributeValues(dataPoints pdata.NumberDataPointSlice, attributeName string) {
	for i := 0; i < dataPoints.Len(); i++ {
		attributes := dataPoints.At(i).Attributes()
		attribute, ok := attributes.Get(attributeName)
		if ok {
			switch attribute.Type() {
			case pdata.ValueTypeString:
				attributes.UpdateString(attributeName, "")
			default:
				panic(fmt.Sprintf("data type not supported: %s", attribute.Type()))
			}
		}
	}
}
