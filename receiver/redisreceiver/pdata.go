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
	"go.opentelemetry.io/collector/consumer/pdata"
)

func newResourceMetrics(ms pdata.MetricSlice, serviceName string) pdata.ResourceMetrics {
	rm := pdata.NewResourceMetrics()
	r := rm.Resource()
	rattrs := r.Attributes()
	rattrs.Insert("type", pdata.NewAttributeValueString(typeStr))
	rattrs.Insert("service.name", pdata.NewAttributeValueString(serviceName))
	ilm := pdata.NewInstrumentationLibraryMetrics()
	rm.InstrumentationLibraryMetrics().Append(ilm)
	ms.CopyTo(ilm.Metrics())
	return rm
}

func buildKeyspaceTriplet(k *keyspace, t *timeBundle) pdata.MetricSlice {
	keysPDM := buildKeyspaceKeysMetric(k, t)
	expiresPDM := buildKeyspaceExpiresMetric(k, t)
	ttlPDM := buildKeyspaceTTLMetric(k, t)

	ms := pdata.NewMetricSlice()
	ms.Append(keysPDM)
	ms.Append(expiresPDM)
	ms.Append(ttlPDM)

	return ms
}

func buildKeyspaceKeysMetric(k *keyspace, t *timeBundle) pdata.Metric {
	m := &redisMetric{
		name:   "redis/db/keys",
		labels: map[string]string{"db": k.db},
		pdType: pdata.MetricDataTypeIntGauge,
	}
	value := int64(k.keys)

	pt := pdata.NewIntDataPoint()
	pt.SetValue(value)
	pdm := newIntMetric(m, pt, t)

	return pdm
}

func buildKeyspaceExpiresMetric(k *keyspace, t *timeBundle) pdata.Metric {
	m := &redisMetric{
		name:   "redis/db/expires",
		labels: map[string]string{"db": k.db},
		pdType: pdata.MetricDataTypeIntGauge,
	}
	value := int64(k.expires)

	pt := pdata.NewIntDataPoint()
	pt.SetValue(value)
	pdm := newIntMetric(m, pt, t)

	return pdm
}

func buildKeyspaceTTLMetric(k *keyspace, t *timeBundle) pdata.Metric {
	m := &redisMetric{
		name:   "redis/db/avg_ttl",
		units:  "ms",
		labels: map[string]string{"db": k.db},
		pdType: pdata.MetricDataTypeIntGauge,
	}
	value := int64(k.avgTTL)

	pt := pdata.NewIntDataPoint()
	pt.SetValue(value)
	pdm := newIntMetric(m, pt, t)

	return pdm
}

func newIntMetric(m *redisMetric, pt pdata.IntDataPoint, t *timeBundle) pdata.Metric {
	pdm := redisMetricToPDM(m)

	pt.SetTimestamp(pdata.TimestampUnixNano(t.current.UnixNano()))
	pt.LabelsMap().InitFromMap(m.labels)

	var points pdata.IntDataPointSlice
	if m.pdType == pdata.MetricDataTypeIntGauge {
		points = pdm.IntGauge().DataPoints()
	} else if m.pdType == pdata.MetricDataTypeIntSum {
		pt.SetStartTime(pdata.TimestampUnixNano(t.serverStart.UnixNano()))
		sum := pdm.IntSum()
		sum.SetIsMonotonic(m.isMonotonic)
		sum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		points = sum.DataPoints()
	}
	points.Append(pt)

	return pdm
}

func newDoubleMetric(m *redisMetric, pt pdata.DoubleDataPoint, t *timeBundle) pdata.Metric {
	pdm := redisMetricToPDM(m)

	pt.SetTimestamp(pdata.TimestampUnixNano(t.current.UnixNano()))
	pt.LabelsMap().InitFromMap(m.labels)

	var points pdata.DoubleDataPointSlice
	if m.pdType == pdata.MetricDataTypeDoubleGauge {
		points = pdm.DoubleGauge().DataPoints()
	} else if m.pdType == pdata.MetricDataTypeDoubleSum {
		pt.SetStartTime(pdata.TimestampUnixNano(t.serverStart.UnixNano()))
		sum := pdm.DoubleSum()
		sum.SetIsMonotonic(m.isMonotonic)
		sum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		points = sum.DataPoints()
	}
	points.Append(pt)

	return pdm
}

func redisMetricToPDM(m *redisMetric) pdata.Metric {
	pdm := pdata.NewMetric()
	pdm.SetDataType(m.pdType)
	pdm.SetName(m.name)
	pdm.SetDescription(m.desc)
	pdm.SetUnit(m.units)
	return pdm
}
