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

// nolint:gocritic
package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var ilm pmetric.ScopeMetrics

func init() {

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	ilm = resourceMetrics.ScopeMetrics().AppendEmpty()

}

// Returns a new Metric of type "Gauge" with specified name and unit
func createGauge(name string, unit string) pmetric.Metric {
	gauge := ilm.Metrics().AppendEmpty()
	gauge.SetDataType(pmetric.MetricDataTypeGauge)
	gauge.SetName(name)
	gauge.SetUnit(unit)
	return gauge
}

// Returns a new Metric of type Monotonic Sum with specified name and unit
func createCounter(name string, unit string) pmetric.Metric {
	counter := ilm.Metrics().AppendEmpty()
	counter.SetDataType(pmetric.MetricDataTypeSum)
	counter.Sum().SetIsMonotonic(true)
	counter.SetName(name)
	counter.SetUnit(unit)
	return counter
}
