// Copyright 2020, OpenTelemetry Authors
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

package redisreceiver

import (
	"go.opentelemetry.io/collector/model/pdata"
)

func buildKeyspaceTriplet(k *keyspace, t *timeBundle) pdata.MetricSlice {
	ms := pdata.NewMetricSlice()
	ms.Resize(3)
	initKeyspaceKeysMetric(k, t, ms.At(0))
	initKeyspaceExpiresMetric(k, t, ms.At(1))
	initKeyspaceTTLMetric(k, t, ms.At(2))
	return ms
}

func initKeyspaceKeysMetric(k *keyspace, t *timeBundle, dest pdata.Metric) {
	m := &redisMetric{
		name:   "redis/db/keys",
		labels: map[string]string{"db": k.db},
		pdType: pdata.MetricDataTypeIntGauge,
	}
	initIntMetric(m, int64(k.keys), t, dest)
}

func initKeyspaceExpiresMetric(k *keyspace, t *timeBundle, dest pdata.Metric) {
	m := &redisMetric{
		name:   "redis/db/expires",
		labels: map[string]string{"db": k.db},
		pdType: pdata.MetricDataTypeIntGauge,
	}
	initIntMetric(m, int64(k.expires), t, dest)
}

func initKeyspaceTTLMetric(k *keyspace, t *timeBundle, dest pdata.Metric) {
	m := &redisMetric{
		name:   "redis/db/avg_ttl",
		units:  "ms",
		labels: map[string]string{"db": k.db},
		pdType: pdata.MetricDataTypeIntGauge,
	}
	initIntMetric(m, int64(k.avgTTL), t, dest)
}

func initIntMetric(m *redisMetric, value int64, t *timeBundle, dest pdata.Metric) {
	redisMetricToPDM(m, dest)

	var pt pdata.IntDataPoint
	if m.pdType == pdata.MetricDataTypeIntGauge {
		pt = dest.IntGauge().DataPoints().AppendEmpty()
	} else if m.pdType == pdata.MetricDataTypeIntSum {
		sum := dest.IntSum()
		sum.SetIsMonotonic(m.isMonotonic)
		sum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		pt = sum.DataPoints().AppendEmpty()
		pt.SetStartTimestamp(pdata.TimestampFromTime(t.serverStart))
	}
	pt.SetValue(value)
	pt.SetTimestamp(pdata.TimestampFromTime(t.current))
	pt.LabelsMap().InitFromMap(m.labels)
}

func initDoubleMetric(m *redisMetric, value float64, t *timeBundle, dest pdata.Metric) {
	redisMetricToPDM(m, dest)

	var pt pdata.DoubleDataPoint
	if m.pdType == pdata.MetricDataTypeDoubleGauge {
		pt = dest.DoubleGauge().DataPoints().AppendEmpty()
	} else if m.pdType == pdata.MetricDataTypeDoubleSum {
		sum := dest.DoubleSum()
		sum.SetIsMonotonic(m.isMonotonic)
		sum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		pt = sum.DataPoints().AppendEmpty()
		pt.SetStartTimestamp(pdata.TimestampFromTime(t.serverStart))
	}
	pt.SetValue(value)
	pt.SetTimestamp(pdata.TimestampFromTime(t.current))
	pt.LabelsMap().InitFromMap(m.labels)
}

func redisMetricToPDM(m *redisMetric, dest pdata.Metric) {
	dest.SetDataType(m.pdType)
	dest.SetName(m.name)
	dest.SetDescription(m.desc)
	dest.SetUnit(m.units)
}
