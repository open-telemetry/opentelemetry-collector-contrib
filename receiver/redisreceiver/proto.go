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
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Helper functions that produce protobuf

func newMetricsData(protoMetrics []*metricspb.Metric, serviceName string) consumerdata.MetricsData {
	return consumerdata.MetricsData{
		Resource: &resourcepb.Resource{
			Type:   typeStr,
			Labels: map[string]string{"service.name": serviceName},
		},
		Metrics: protoMetrics,
	}
}

func buildKeyspaceTriplet(k *keyspace, t *timeBundle) []*metricspb.Metric {
	return []*metricspb.Metric{
		buildKeyspaceKeysMetric(k, t),
		buildKeyspaceExpiresMetric(k, t),
		buildKeyspaceTTLMetric(k, t),
	}
}

func buildKeyspaceKeysMetric(k *keyspace, t *timeBundle) *metricspb.Metric {
	m := &redisMetric{
		name:   "redis/db/keys",
		labels: map[string]string{"db": k.db},
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
	pt := &metricspb.Point{Value: &metricspb.Point_Int64Value{Int64Value: int64(k.keys)}}
	return newProtoMetric(m, pt, t)
}

func buildKeyspaceExpiresMetric(k *keyspace, t *timeBundle) *metricspb.Metric {
	m := &redisMetric{
		name:   "redis/db/expires",
		labels: map[string]string{"db": k.db},
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
	pt := &metricspb.Point{Value: &metricspb.Point_Int64Value{Int64Value: int64(k.expires)}}
	return newProtoMetric(m, pt, t)
}

func buildKeyspaceTTLMetric(k *keyspace, t *timeBundle) *metricspb.Metric {
	m := &redisMetric{
		name:   "redis/db/avg_ttl",
		units:  "ms",
		labels: map[string]string{"db": k.db},
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
	pt := &metricspb.Point{Value: &metricspb.Point_Int64Value{Int64Value: int64(k.avgTTL)}}
	return newProtoMetric(m, pt, t)
}

// Create new protobuf Metric.
// Arguments:
//   * redisMetric -- the fixed metadata to build the protobuf metric
//   * pt -- the protobuf Point to be wrapped
//   * currTime -- the timestamp to be put on the Point (not on the timeseries)
func newProtoMetric(m *redisMetric, pt *metricspb.Point, t *timeBundle) *metricspb.Metric {
	var startTime *timestamppb.Timestamp

	if m.mdType == metricspb.MetricDescriptor_CUMULATIVE_INT64 ||
		m.mdType == metricspb.MetricDescriptor_CUMULATIVE_DOUBLE {
		startTime = timestamppb.New(t.serverStart)
	}

	pt.Timestamp = timestamppb.New(t.current)
	labelKeys, labelVals := buildLabels(m.labels, m.labelDescriptions)
	pbMetric := &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:        m.name,
			Description: m.desc,
			Unit:        m.units,
			Type:        m.mdType,
			LabelKeys:   labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{{
			LabelValues:    labelVals,
			Points:         []*metricspb.Point{pt},
			StartTimestamp: startTime,
		}},
	}
	return pbMetric
}

func buildLabels(labels map[string]string, descriptions map[string]string) (
	[]*metricspb.LabelKey, []*metricspb.LabelValue,
) {
	var keys []*metricspb.LabelKey
	var values []*metricspb.LabelValue
	for key, val := range labels {
		labelKey := &metricspb.LabelKey{Key: key}
		desc, hasDesc := descriptions[key]
		if hasDesc {
			labelKey.Description = desc
		}
		keys = append(keys, labelKey)
		values = append(values, &metricspb.LabelValue{
			Value:    val,
			HasValue: true,
		})
	}
	return keys, values
}
