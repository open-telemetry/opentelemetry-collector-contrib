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

package filterexpr

import (
	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"go.opentelemetry.io/collector/model/pdata"
)

type Matcher struct {
	program *vm.Program
	v       vm.VM
}

type env struct {
	MetricName string

	// TODO: replace this with GetAttribute func(key string) (string,bool)
	HasAttribute func(key string) bool
	Attribute    func(key string) string

	// HasLabel and Label are aliases for HasAttribute and Attribute, kept for
	// backwards compatibility. These are deprecated and will be removed in the future.
	HasLabel func(key string) bool
	Label    func(key string) string
}

func NewMatcher(expression string) (*Matcher, error) {
	program, err := expr.Compile(expression)
	if err != nil {
		return nil, err
	}
	return &Matcher{program: program, v: vm.VM{}}, nil
}

func (m *Matcher) MatchMetric(metric pdata.Metric) (bool, error) {
	metricName := metric.Name()
	switch metric.DataType() {
	case pdata.MetricDataTypeGauge:
		return m.matchGauge(metricName, metric.Gauge())
	case pdata.MetricDataTypeSum:
		return m.matchSum(metricName, metric.Sum())
	case pdata.MetricDataTypeHistogram:
		return m.matchDoubleHistogram(metricName, metric.Histogram())
	default:
		return false, nil
	}
}

func (m *Matcher) matchGauge(metricName string, gauge pdata.Gauge) (bool, error) {
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

func (m *Matcher) matchSum(metricName string, sum pdata.Sum) (bool, error) {
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

func (m *Matcher) matchDoubleHistogram(metricName string, histogram pdata.Histogram) (bool, error) {
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

func (m *Matcher) matchEnv(metricName string, attributes pdata.AttributeMap) (bool, error) {
	return m.match(createEnv(metricName, attributes))
}

func createEnv(metricName string, attributes pdata.AttributeMap) env {
	e := env{
		MetricName: metricName,
		HasAttribute: func(key string) bool {
			_, ok := attributes.Get(key)
			return ok
		},
		Attribute: func(key string) string {
			v, _ := attributes.Get(key)
			return v.StringVal()
		},
	}
	// HasLabel and Label are aliases for HasAttribute and Attribute.
	e.HasLabel = e.HasAttribute
	e.Label = e.Attribute
	return e
}

func (m *Matcher) match(env env) (bool, error) {
	result, err := m.v.Run(m.program, env)
	if err != nil {
		return false, err
	}
	return result.(bool), nil
}
