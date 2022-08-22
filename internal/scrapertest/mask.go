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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// IgnoreMetricValues is a CompareOption that clears all values
func IgnoreMetricValues() CompareOption {
	return ignoreMetricValues{}
}

type ignoreMetricValues struct{}

func (opt ignoreMetricValues) apply(expected, actual pmetric.Metrics) {
	maskMetricValues(expected)
	maskMetricValues(actual)
}

func maskMetricValues(metrics pmetric.Metrics) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			maskMetricSliceValues(ilms.At(j).Metrics())
		}
	}
}

// maskMetricSliceValues sets all data point values to zero.
func maskMetricSliceValues(metrics pmetric.MetricSlice) {
	for i := 0; i < metrics.Len(); i++ {
		maskDataPointSliceValues(getDataPointSlice(metrics.At(i)))
	}
}

// maskDataPointSliceValues sets all data point values to zero.
func maskDataPointSliceValues(dataPoints pmetric.NumberDataPointSlice) {
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

func (opt ignoreMetricAttributeValue) apply(expected, actual pmetric.Metrics) {
	maskMetricAttributeValue(expected, opt)
	maskMetricAttributeValue(actual, opt)
}

func maskMetricAttributeValue(metrics pmetric.Metrics, opt ignoreMetricAttributeValue) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			maskMetricSliceAttributeValues(ilms.At(j).Metrics(), opt.attributeName, opt.metricNames...)
		}
	}
}

// maskMetricSliceAttributeValues sets the value of the specified attribute to
// the zero value associated with the attribute data type.
// If metric names are specified, only the data points within those metrics will be masked.
// Otherwise, all data points with the attribute will be masked.
func maskMetricSliceAttributeValues(metrics pmetric.MetricSlice, attributeName string, metricNames ...string) {
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
			dps.Sort(func(a, b pmetric.NumberDataPoint) bool {
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
func maskDataPointSliceAttributeValues(dataPoints pmetric.NumberDataPointSlice, attributeName string) {
	for i := 0; i < dataPoints.Len(); i++ {
		attributes := dataPoints.At(i).Attributes()
		attribute, ok := attributes.Get(attributeName)
		if ok {
			switch attribute.Type() {
			case pcommon.ValueTypeString:
				attributes.UpdateString(attributeName, "")
			default:
				panic(fmt.Sprintf("data type not supported: %s", attribute.Type()))
			}
		}
	}
}

// IgnoreResourceAttributeValue is a CompareOption that removes a resource attribute
// from all resources
func IgnoreResourceAttributeValue(attributeName string) CompareOption {
	return ignoreResourceAttributeValue{
		attributeName: attributeName,
	}
}

type ignoreResourceAttributeValue struct {
	attributeName string
}

func (opt ignoreResourceAttributeValue) apply(expected, actual pmetric.Metrics) {
	maskResourceAttributeValue(expected, opt)
	maskResourceAttributeValue(actual, opt)
}

func maskResourceAttributeValue(metrics pmetric.Metrics, opt ignoreResourceAttributeValue) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		if _, ok := rms.At(i).Resource().Attributes().Get(opt.attributeName); ok {
			rms.At(i).Resource().Attributes().Remove(opt.attributeName)
		}
	}
}
