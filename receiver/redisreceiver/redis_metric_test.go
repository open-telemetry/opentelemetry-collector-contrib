package redisreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildSingleProtoMetric_PointTimestamp(t *testing.T) {
	now := time.Now()
	metric, err := buildSingleProtoMetric(uptimeInSeconds(), "42", now)
	require.Nil(t, err)
	ts := metric.Timeseries[0]
	pt := ts.Points[0]
	require.Equal(t, now.Unix(), pt.Timestamp.GetSeconds())
	nanosecond := now.Nanosecond()
	require.Equal(t, int32(nanosecond), pt.Timestamp.GetNanos())
}

func TestBuildSingleProtoMetric_Labels(t *testing.T) {
	metric, err := buildSingleProtoMetric(usedCpuSys(), "42", time.Time{})
	require.Nil(t, err)
	require.Equal(t, 1, len(metric.MetricDescriptor.LabelKeys))
	key := metric.MetricDescriptor.LabelKeys[0].Key
	require.Equal(t, "state", key)
	require.Equal(t, 1, len(metric.Timeseries[0].LabelValues))
	require.Equal(t, "sys", metric.Timeseries[0].LabelValues[0].Value)
}
