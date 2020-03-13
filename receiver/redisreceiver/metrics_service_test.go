package redisreceiver

import (
	"testing"

	metricsProto "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMemoryMetric(t *testing.T) {
	md := getMetricData(t, usedMemory())
	require.Equal(t, 1, len(md.Metrics))
	require.NotNil(t, md.Resource)
	metric := md.Metrics[0]
	require.Equal(t, "used_memory", metric.MetricDescriptor.Name)
	require.Equal(t, "memory used", metric.MetricDescriptor.Description)
	require.Equal(t, "bytes", metric.MetricDescriptor.Unit)
	require.Equal(t, metricsProto.MetricDescriptor_GAUGE_INT64, metric.MetricDescriptor.Type)
	requireIntPtEqual(t, 854160, metric)
}

func TestUptimeInSeconds(t *testing.T) {
	metric := getProtoMetric(t, uptimeInSeconds())
	require.Equal(t, "seconds", metric.MetricDescriptor.Unit)
	requireIntPtEqual(t, 104946, metric)
}

func TestUsedCpuSys(t *testing.T) {
	md := getMetricData(t, usedCpuSys())
	metric := md.Metrics[0]
	require.Equal(t, metricsProto.MetricDescriptor_GAUGE_DOUBLE, metric.MetricDescriptor.Type)
	require.Equal(t, "mhz", metric.MetricDescriptor.Unit)
	requireDoublePtEqual(t, 185.649184, metric)
}

func TestMissingMetricValue(t *testing.T) {
	md := getMetricData(t, usedCpuSysUserChildren())
	require.Nil(t, md.Metrics)
}

func TestAllMetrics(t *testing.T) {
	svc := newMetricsService(newFakeClient(), nil)
	md, err := svc.getMetricsData(getDefaultRedisMetrics())
	require.Nil(t, err)
	require.Equal(t, 29, len(md.Metrics))
}

func getProtoMetric(t *testing.T, redisMetric *redisMetric) *metricsProto.Metric {
	md := getMetricData(t, redisMetric)
	metric := md.Metrics[0]
	return metric
}

func getMetricData(t *testing.T, metric *redisMetric) *consumerdata.MetricsData {
	logger, _ := zap.NewDevelopment()
	svc := newMetricsService(newFakeClient(), logger)
	md, err := svc.getMetricsData([]*redisMetric{metric})
	require.Nil(t, err)
	return md
}

func requireIntPtEqual(t *testing.T, i int64, metric *metricsProto.Metric) {
	require.Equal(t, &metricsProto.Point_Int64Value{Int64Value: i}, metric.Timeseries[0].Points[0].Value)
}

func requireDoublePtEqual(t *testing.T, f float64, metric *metricsProto.Metric) {
	require.Equal(t, &metricsProto.Point_DoubleValue{DoubleValue: f}, metric.Timeseries[0].Points[0].Value)
}
