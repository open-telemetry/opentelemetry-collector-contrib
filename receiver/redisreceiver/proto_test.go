package redisreceiver

import (
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/require"
)

func TestMemoryMetric(t *testing.T) {
	md := getMetricData(t, usedMemory())
	require.Equal(t, 1, len(md.Metrics))
	require.NotNil(t, md.Resource)
	metric := md.Metrics[0]
	require.Equal(t, "redis/memory/used", metric.MetricDescriptor.Name)
	require.Equal(t, "memory used", metric.MetricDescriptor.Description)
	require.Equal(t, "By", metric.MetricDescriptor.Unit)
	require.Equal(t, metricspb.MetricDescriptor_GAUGE_INT64, metric.MetricDescriptor.Type)
	requireIntPtEqual(t, 854160, metric)
}

func TestUptimeInSeconds(t *testing.T) {
	metric := getProtoMetric(t, uptimeInSeconds())
	require.Equal(t, "s", metric.MetricDescriptor.Unit)
	requireIntPtEqual(t, 104946, metric)
}

func TestUsedCpuSys(t *testing.T) {
	md := getMetricData(t, usedCpuSys())
	metric := md.Metrics[0]
	require.Equal(t, metricspb.MetricDescriptor_GAUGE_DOUBLE, metric.MetricDescriptor.Type)
	require.Equal(t, "s", metric.MetricDescriptor.Unit)
	requireDoublePtEqual(t, 185.649184, metric)
}

func TestMissingMetricValue(t *testing.T) {
	redisMetrics := []*redisMetric{{key: "config_file"}}
	_, err, warnings := fetchMetrics(redisMetrics)
	require.Nil(t, err)
	// treat a missing value as not worthy of a warning
	require.Nil(t, warnings)
}

func TestMissingMetric(t *testing.T) {
	redisMetrics := []*redisMetric{{key: "foo"}}
	_, err, warnings := fetchMetrics(redisMetrics)
	require.Nil(t, err)
	require.Equal(t, 1, len(warnings))
}

func TestAllMetrics(t *testing.T) {
	redisMetrics := getDefaultRedisMetrics()
	protoMetrics, err, warnings := fetchMetrics(redisMetrics)
	require.Nil(t, err)
	require.Nil(t, warnings)
	require.Equal(t, len(redisMetrics), len(protoMetrics))
}

func TestKeyspaceMetrics(t *testing.T) {
	svc := newRedisSvc(newFakeClient())
	info, _ := svc.info()
	m, err := buildKeyspaceProtoMetrics(info, time.Time{})
	require.Nil(t, err)

	metric := m[0]
	require.Equal(t, "redis/db/keys", metric.MetricDescriptor.Name)
	require.Equal(t, "db", metric.MetricDescriptor.LabelKeys[0].Key)
	require.Equal(t, "0", metric.Timeseries[0].LabelValues[0].Value)
	require.Equal(t, metricspb.MetricDescriptor_CUMULATIVE_INT64, metric.MetricDescriptor.Type)
	require.Equal(t, &metricspb.Point_Int64Value{Int64Value: 1}, metric.Timeseries[0].Points[0].Value)

	metric = m[1]
	require.Equal(t, "redis/db/expires", metric.MetricDescriptor.Name)
	require.Equal(t, "db", metric.MetricDescriptor.LabelKeys[0].Key)
	require.Equal(t, "0", metric.Timeseries[0].LabelValues[0].Value)
	require.Equal(t, metricspb.MetricDescriptor_CUMULATIVE_INT64, metric.MetricDescriptor.Type)
	require.Equal(t, &metricspb.Point_Int64Value{Int64Value: 2}, metric.Timeseries[0].Points[0].Value)

	metric = m[2]
	require.Equal(t, "redis/db/avg_ttl", metric.MetricDescriptor.Name)
	require.Equal(t, "db", metric.MetricDescriptor.LabelKeys[0].Key)
	require.Equal(t, "0", metric.Timeseries[0].LabelValues[0].Value)
	require.Equal(t, metricspb.MetricDescriptor_CUMULATIVE_INT64, metric.MetricDescriptor.Type)
	require.Equal(t, &metricspb.Point_Int64Value{Int64Value: 3}, metric.Timeseries[0].Points[0].Value)
}

func fetchMetrics(redisMetrics []*redisMetric) ([]*metricspb.Metric, error, []error) {
	svc := newRedisSvc(newFakeClient())
	info, err := svc.info()
	protoMetrics, warnings := buildFixedProtoMetrics(info, redisMetrics, time.Time{})
	return protoMetrics, err, warnings
}

func getProtoMetric(t *testing.T, redisMetric *redisMetric) *metricspb.Metric {
	md := getMetricData(t, redisMetric)
	metric := md.Metrics[0]
	return metric
}

func getMetricData(t *testing.T, metric *redisMetric) *consumerdata.MetricsData {
	md, err, warnings := getMetricDataErr(metric)
	require.Nil(t, err)
	require.Nil(t, warnings)
	return md
}

func getMetricDataErr(metric *redisMetric) (*consumerdata.MetricsData, error, []error) {
	redisMetrics := []*redisMetric{metric}
	svc := newRedisSvc(newFakeClient())
	info, err := svc.info()
	protoMetrics, warnings := buildFixedProtoMetrics(info, redisMetrics, time.Time{})
	md := newMetricsData(protoMetrics)
	return md, err, warnings
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
