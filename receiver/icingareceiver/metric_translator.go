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

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func buildCounterMetric(parsedMetric icingaMetric, timeNow time.Time) pmetric.ScopeMetrics {
	ilm := pmetric.NewScopeMetrics()
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}
	nm.SetDataType(pmetric.MetricDataTypeSum)

	nm.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	nm.Sum().SetIsMonotonic(true)

	dp := nm.Sum().DataPoints().AppendEmpty()
	dp.SetIntVal(parsedMetric.counterValue())
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timeNow))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
	for i := parsedMetric.description.attrs.Iter(); i.Next(); {
		dp.Attributes().InsertString(string(i.Attribute().Key), i.Attribute().Value.AsString())
	}

	return ilm
}

func buildGaugeMetric(parsedMetric icingaMetric, timeNow time.Time) pmetric.ScopeMetrics {
	ilm := pmetric.NewScopeMetrics()
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}
	nm.SetDataType(pmetric.MetricDataTypeGauge)
	dp := nm.Gauge().DataPoints().AppendEmpty()
	dp.SetDoubleVal(parsedMetric.gaugeValue())
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
	for i := parsedMetric.description.attrs.Iter(); i.Next(); {
		dp.Attributes().InsertString(string(i.Attribute().Key), i.Attribute().Value.AsString())
	}

	return ilm
}

func (s icingaMetric) counterValue() int64 {
	return int64(s.asFloat)
}

func (s icingaMetric) gaugeValue() float64 {
	return s.asFloat
}
