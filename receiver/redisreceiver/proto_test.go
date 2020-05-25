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
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
)

func TestServiceName(t *testing.T) {
	const serviceName = "foo-service"
	md := getMetricData(t, usedMemory(), serviceName)
	val := md.Resource.Labels["service.name"]
	require.Equal(t, serviceName, val)
}

func TestMemoryMetric(t *testing.T) {
	md := getMetricData(t, usedMemory(), "")
	require.Equal(t, 1, len(md.Metrics))
	require.NotNil(t, md.Resource)
	metric := md.Metrics[0]
	require.Equal(t, "redis/memory/used", metric.MetricDescriptor.Name)
	require.Equal(
		t,
		"Total number of bytes allocated by Redis using its allocator",
		metric.MetricDescriptor.Description,
	)
	require.Equal(t, "By", metric.MetricDescriptor.Unit)
	require.Equal(t, metricspb.MetricDescriptor_GAUGE_INT64, metric.MetricDescriptor.Type)
	requireIntPtEqual(t, 854160, metric)
}

func TestUptimeInSeconds(t *testing.T) {
	metric := getProtoMetric(t, uptimeInSeconds(), "")
	require.Equal(t, "s", metric.MetricDescriptor.Unit)
	requireIntPtEqual(t, 104946, metric)
}

func TestUsedCpuSys(t *testing.T) {
	md := getMetricData(t, usedCPUSys(), "")
	metric := md.Metrics[0]
	require.Equal(t, metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metric.MetricDescriptor.Type)
	require.Equal(t, "s", metric.MetricDescriptor.Unit)
	requireDoublePtEqual(t, 185.649184, metric)
}

func TestMissingMetricValue(t *testing.T) {
	redisMetrics := []*redisMetric{{key: "config_file"}}
	_, warnings, err := fetchMetrics(redisMetrics)
	require.Nil(t, err)
	// treat a missing value as not worthy of a warning
	require.Nil(t, warnings)
}

func TestMissingMetric(t *testing.T) {
	redisMetrics := []*redisMetric{{key: "foo"}}
	_, warnings, err := fetchMetrics(redisMetrics)
	require.Nil(t, err)
	require.Equal(t, 1, len(warnings))
}

func TestAllMetrics(t *testing.T) {
	redisMetrics := getDefaultRedisMetrics()
	protoMetrics, warnings, err := fetchMetrics(redisMetrics)
	require.Nil(t, err)
	require.Nil(t, warnings)
	require.Equal(t, len(redisMetrics), len(protoMetrics))
}

func TestKeyspaceMetrics(t *testing.T) {
	svc := newRedisSvc(newFakeClient())
	info, _ := svc.info()
	m, err := info.buildKeyspaceProtoMetrics(getDefaultTimeBundle())
	require.Nil(t, err)

	metric := m[0]
	require.Equal(t, "redis/db/keys", metric.MetricDescriptor.Name)
	require.Equal(t, "db", metric.MetricDescriptor.LabelKeys[0].Key)
	require.Equal(t, "0", metric.Timeseries[0].LabelValues[0].Value)
	require.Equal(t, metricspb.MetricDescriptor_GAUGE_INT64, metric.MetricDescriptor.Type)
	require.Equal(t, &metricspb.Point_Int64Value{Int64Value: 1}, metric.Timeseries[0].Points[0].Value)

	metric = m[1]
	require.Equal(t, "redis/db/expires", metric.MetricDescriptor.Name)
	require.Equal(t, "db", metric.MetricDescriptor.LabelKeys[0].Key)
	require.Equal(t, "0", metric.Timeseries[0].LabelValues[0].Value)
	require.Equal(t, metricspb.MetricDescriptor_GAUGE_INT64, metric.MetricDescriptor.Type)
	require.Equal(t, &metricspb.Point_Int64Value{Int64Value: 2}, metric.Timeseries[0].Points[0].Value)

	metric = m[2]
	require.Equal(t, "redis/db/avg_ttl", metric.MetricDescriptor.Name)
	require.Equal(t, "db", metric.MetricDescriptor.LabelKeys[0].Key)
	require.Equal(t, "0", metric.Timeseries[0].LabelValues[0].Value)
	require.Equal(t, metricspb.MetricDescriptor_GAUGE_INT64, metric.MetricDescriptor.Type)
	require.Equal(t, &metricspb.Point_Int64Value{Int64Value: 3}, metric.Timeseries[0].Points[0].Value)
}

func TestNewProtoMetric(t *testing.T) {
	serverStartTime, _ := ptypes.TimestampProto(time.Unix(900, 0))
	tests := []struct {
		name     string
		mdType   metricspb.MetricDescriptor_Type
		expected *timestamp.Timestamp
	}{
		{"cumulative int", metricspb.MetricDescriptor_CUMULATIVE_INT64, serverStartTime},
		{"cumulative double", metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, serverStartTime},
		{"gauge int", metricspb.MetricDescriptor_GAUGE_INT64, nil},
		{"gauge double", metricspb.MetricDescriptor_GAUGE_DOUBLE, nil},
	}
	bundle := getDefaultTimeBundle()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := &redisMetric{mdType: test.mdType}
			proto := newProtoMetric(m, &metricspb.Point{}, bundle)
			require.Equal(t, test.expected, proto.Timeseries[0].StartTimestamp)
		})
	}
}

func fetchMetrics(redisMetrics []*redisMetric) ([]*metricspb.Metric, []error, error) {
	svc := newRedisSvc(newFakeClient())
	info, err := svc.info()
	if err != nil {
		return nil, nil, err
	}
	protoMetrics, warnings := info.buildFixedProtoMetrics(redisMetrics, getDefaultTimeBundle())
	return protoMetrics, warnings, nil
}

func getProtoMetric(t *testing.T, redisMetric *redisMetric, serverName string) *metricspb.Metric {
	md := getMetricData(t, redisMetric, serverName)
	metric := md.Metrics[0]
	return metric
}

func getMetricData(t *testing.T, metric *redisMetric, serverName string) *consumerdata.MetricsData {
	md, warnings, err := getMetricDataErr(metric, serverName)
	require.Nil(t, err)
	require.Nil(t, warnings)
	return md
}

func getMetricDataErr(metric *redisMetric, serverName string) (*consumerdata.MetricsData, []error, error) {
	redisMetrics := []*redisMetric{metric}
	svc := newRedisSvc(newFakeClient())
	info, err := svc.info()
	if err != nil {
		return nil, nil, err
	}
	protoMetrics, warnings := info.buildFixedProtoMetrics(redisMetrics, getDefaultTimeBundle())
	md := newMetricsData(protoMetrics, serverName)
	return md, warnings, nil
}

func getDefaultTimeBundle() *timeBundle {
	return newTimeBundle(time.Unix(1000, 0), 100)
}

func requireIntPtEqual(t *testing.T, i int64, metric *metricspb.Metric) {
	require.Equal(
		t,
		&metricspb.Point_Int64Value{Int64Value: i},
		metric.Timeseries[0].Points[0].Value,
	)
}

func requireDoublePtEqual(t *testing.T, f float64, metric *metricspb.Metric) {
	require.Equal(
		t,
		&metricspb.Point_DoubleValue{DoubleValue: f},
		metric.Timeseries[0].Points[0].Value,
	)
}
