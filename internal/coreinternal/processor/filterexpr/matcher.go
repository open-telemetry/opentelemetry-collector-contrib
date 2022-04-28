// Copyright The OpenTelemetry Authors
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

package filterexpr // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterexpr"

import (
	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Matcher struct {
	program *vm.Program
	v       vm.VM
}

type env struct {
	MetricName string
	attributes pcommon.Map
}

func (e *env) HasLabel(key string) bool {
	_, ok := e.attributes.Get(key)
	return ok
}

func (e *env) Label(key string) string {
	v, _ := e.attributes.Get(key)
	return v.StringVal()
}

func NewMatcher(expression string) (*Matcher, error) {
	program, err := expr.Compile(expression)
	if err != nil {
		return nil, err
	}
	return &Matcher{program: program, v: vm.VM{}}, nil
}

func (m *Matcher) MatchMetric(metric pmetric.Metric) (bool, error) {
	metricName := metric.Name()
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		return m.matchGauge(metricName, metric.Gauge())
	case pmetric.MetricDataTypeSum:
		return m.matchSum(metricName, metric.Sum())
	case pmetric.MetricDataTypeHistogram:
		return m.matchDoubleHistogram(metricName, metric.Histogram())
	default:
		return false, nil
	}
}

func (m *Matcher) matchGauge(metricName string, gauge pmetric.Gauge) (bool, error) {
	pts := gauge.DataPoints()
	for i := 0; i < pts.Len(); i++ {
		matched, err := m.matchEnv(metricName, pts.At(i).Attributes())
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (m *Matcher) matchSum(metricName string, sum pmetric.Sum) (bool, error) {
	pts := sum.DataPoints()
	for i := 0; i < pts.Len(); i++ {
		matched, err := m.matchEnv(metricName, pts.At(i).Attributes())
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (m *Matcher) matchDoubleHistogram(metricName string, histogram pmetric.Histogram) (bool, error) {
	pts := histogram.DataPoints()
	for i := 0; i < pts.Len(); i++ {
		matched, err := m.matchEnv(metricName, pts.At(i).Attributes())
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (m *Matcher) matchEnv(metricName string, attributes pcommon.Map) (bool, error) {
	return m.match(createEnv(metricName, attributes))
}

func createEnv(metricName string, attributes pcommon.Map) *env {
	return &env{
		MetricName: metricName,
		attributes: attributes,
	}
}

func (m *Matcher) match(env *env) (bool, error) {
	result, err := m.v.Run(m.program, env)
	if err != nil {
		return false, err
	}
	return result.(bool), nil
}
