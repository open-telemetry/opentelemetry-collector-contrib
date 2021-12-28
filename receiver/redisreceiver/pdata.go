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

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"go.opentelemetry.io/collector/model/pdata"
)

func buildKeyspaceTriplet(k *keyspace, t *timeBundle) pdata.MetricSlice {
	ms := pdata.NewMetricSlice()
	ms.EnsureCapacity(3)
	initKeyspaceKeysMetric(k, t, ms.AppendEmpty())
	initKeyspaceExpiresMetric(k, t, ms.AppendEmpty())
	initKeyspaceTTLMetric(k, t, ms.AppendEmpty())
	return ms
}

func initKeyspaceKeysMetric(k *keyspace, t *timeBundle, dest pdata.Metric) {
	m := &redisMetric{
		name:   "redis.db.keys",
		labels: map[string]pdata.AttributeValue{"db": pdata.NewAttributeValueString(k.db)},
		pdType: pdata.MetricDataTypeGauge,
		desc:   "Number of keyspace keys",
	}
	initIntMetric(m, int64(k.keys), t, dest)
}

func initKeyspaceExpiresMetric(k *keyspace, t *timeBundle, dest pdata.Metric) {
	m := &redisMetric{
		name:   "redis.db.expires",
		labels: map[string]pdata.AttributeValue{"db": pdata.NewAttributeValueString(k.db)},
		pdType: pdata.MetricDataTypeGauge,
		desc:   "Number of keyspace keys with an expiration",
	}
	initIntMetric(m, int64(k.expires), t, dest)
}

func initKeyspaceTTLMetric(k *keyspace, t *timeBundle, dest pdata.Metric) {
	m := &redisMetric{
		name:   "redis.db.avg_ttl",
		units:  "ms",
		labels: map[string]pdata.AttributeValue{"db": pdata.NewAttributeValueString(k.db)},
		pdType: pdata.MetricDataTypeGauge,
		desc:   "Average keyspace keys TTL",
	}
	initIntMetric(m, int64(k.avgTTL), t, dest)
}

func initIntMetric(m *redisMetric, value int64, t *timeBundle, dest pdata.Metric) {
	redisMetricToPDM(m, dest)

	var pt pdata.NumberDataPoint
	if m.pdType == pdata.MetricDataTypeGauge {
		pt = dest.Gauge().DataPoints().AppendEmpty()
	} else if m.pdType == pdata.MetricDataTypeSum {
		sum := dest.Sum()
		sum.SetIsMonotonic(m.isMonotonic)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
		pt = sum.DataPoints().AppendEmpty()
		pt.SetStartTimestamp(pdata.NewTimestampFromTime(t.serverStart))
	}
	pt.SetIntVal(value)
	pt.SetTimestamp(pdata.NewTimestampFromTime(t.current))
	pdata.NewAttributeMapFromMap(m.labels).CopyTo(pt.Attributes())
}

func initDoubleMetric(m *redisMetric, value float64, t *timeBundle, dest pdata.Metric) {
	redisMetricToPDM(m, dest)

	var pt pdata.NumberDataPoint
	if m.pdType == pdata.MetricDataTypeGauge {
		pt = dest.Gauge().DataPoints().AppendEmpty()
	} else if m.pdType == pdata.MetricDataTypeSum {
		sum := dest.Sum()
		sum.SetIsMonotonic(m.isMonotonic)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
		pt = sum.DataPoints().AppendEmpty()
		pt.SetStartTimestamp(pdata.NewTimestampFromTime(t.serverStart))
	}
	pt.SetDoubleVal(value)
	pt.SetTimestamp(pdata.NewTimestampFromTime(t.current))
	pdata.NewAttributeMapFromMap(m.labels).CopyTo(pt.Attributes())
}

func redisMetricToPDM(m *redisMetric, dest pdata.Metric) {
	dest.SetDataType(m.pdType)
	dest.SetName(m.name)
	dest.SetDescription(m.desc)
	dest.SetUnit(m.units)
}
