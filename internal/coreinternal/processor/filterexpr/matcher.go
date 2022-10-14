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
	"fmt"

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
	MetricType string
	attributes pcommon.Map
}

func (e *env) HasLabel(key string) bool {
	_, ok := e.attributes.Get(key)
	return ok
}

func (e *env) Label(key string) string {
	v, _ := e.attributes.Get(key)
	return v.Str()
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
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return m.matchGauge(metricName, metric.Gauge())
	case pmetric.MetricTypeSum:
		return m.matchSum(metricName, metric.Sum())
	case pmetric.MetricTypeHistogram:
		return m.matchHistogram(metricName, metric.Histogram())
	case pmetric.MetricTypeExponentialHistogram:
		return m.matchExponentialHistogram(metricName, metric.ExponentialHistogram())
	case pmetric.MetricTypeSummary:
		return m.matchSummary(metricName, metric.Summary())
	default:
		return false, nil
	}
}

func (m *Matcher) matchGauge(metricName string, gauge pmetric.Gauge) (bool, error) {
	pts := gauge.DataPoints()
	for i := 0; i < pts.Len(); i++ {
		matched, err := m.matchEnv(metricName, pmetric.MetricTypeGauge, pts.At(i).Attributes())
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
		matched, err := m.matchEnv(metricName, pmetric.MetricTypeSum, pts.At(i).Attributes())
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (m *Matcher) matchHistogram(metricName string, histogram pmetric.Histogram) (bool, error) {
	pts := histogram.DataPoints()
	for i := 0; i < pts.Len(); i++ {
		matched, err := m.matchEnv(metricName, pmetric.MetricTypeHistogram, pts.At(i).Attributes())
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (m *Matcher) matchExponentialHistogram(metricName string, eh pmetric.ExponentialHistogram) (bool, error) {
	pts := eh.DataPoints()
	for i := 0; i < pts.Len(); i++ {
		matched, err := m.matchEnv(metricName, pmetric.MetricTypeExponentialHistogram, pts.At(i).Attributes())
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (m *Matcher) matchSummary(metricName string, summary pmetric.Summary) (bool, error) {
	pts := summary.DataPoints()
	for i := 0; i < pts.Len(); i++ {
		matched, err := m.matchEnv(metricName, pmetric.MetricTypeSummary, pts.At(i).Attributes())
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (m *Matcher) matchEnv(metricName string, metricType pmetric.MetricType, attributes pcommon.Map) (bool, error) {
	return m.match(env{
		MetricName: metricName,
		MetricType: metricType.String(),
		attributes: attributes,
	})
}

func (m *Matcher) match(env env) (bool, error) {
	result, err := m.v.Run(m.program, &env)
	if err != nil {
		return false, err
	}

	v, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("filter returned non-boolean value type=%T result=%v metric=%s, attributes=%v",
			result, result, env.MetricName, env.attributes.AsRaw())
	}

	return v, nil
}
