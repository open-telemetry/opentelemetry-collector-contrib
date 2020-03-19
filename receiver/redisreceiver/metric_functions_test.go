package redisreceiver

import (
	"strings"
	"testing"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
)

func TestDefaultMetrics(t *testing.T) {
	for _, metric := range getDefaultRedisMetrics() {
		require.True(t, len(metric.key) > 0)
		require.True(t, len(metric.name) > 0)
		require.True(t, strings.HasPrefix(metric.name, "redis/"))
		require.True(
			t,
			metric.mdType == v1.MetricDescriptor_GAUGE_INT64 ||
				metric.mdType == v1.MetricDescriptor_GAUGE_DOUBLE ||
				metric.mdType == v1.MetricDescriptor_CUMULATIVE_INT64 ||
				metric.mdType == v1.MetricDescriptor_CUMULATIVE_DOUBLE,
		)
	}
}
