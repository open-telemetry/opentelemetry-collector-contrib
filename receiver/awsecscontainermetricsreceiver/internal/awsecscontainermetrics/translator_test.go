// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awsecscontainermetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestConvertToOTMetrics(t *testing.T) {
	now := time.Now()
	timestamp := pcommon.NewTimestampFromTime(now)
	startTimestamp := pcommon.NewTimestampFromTime(now.Add(-10 * time.Minute))
	m := ECSMetrics{}

	m.MemoryUsage = 100
	m.MemoryMaxUsage = 100
	m.MemoryUtilized = 100
	m.MemoryReserved = 100
	m.CPUTotalUsage = 100

	resource := pcommon.NewResource()
	md := convertToOTLPMetrics("container.", m, resource, timestamp, startTimestamp)
	require.Equal(t, 26, md.ResourceMetrics().At(0).ScopeMetrics().Len())
	assert.Contains(t, md.ResourceMetrics().At(0).SchemaUrl(), "https://opentelemetry.io/schemas/")

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		ms := scopeMetrics.At(i).Metrics()
		for j := 0; j < ms.Len(); j++ {
			metric := ms.At(j)
			if metric.Type() == pmetric.MetricTypeSum {
				assert.True(t, metric.Sum().IsMonotonic())
				assert.Equal(t, pmetric.AggregationTemporalityCumulative, metric.Sum().AggregationTemporality())
				assert.Equal(t, startTimestamp, metric.Sum().DataPoints().At(0).StartTimestamp())
				assert.Equal(t, timestamp, metric.Sum().DataPoints().At(0).Timestamp())
			}
		}
	}
}

func TestConvertToOTMetricsTaskLevel(t *testing.T) {
	now := time.Now()
	timestamp := pcommon.NewTimestampFromTime(now)
	startTimestamp := pcommon.NewTimestampFromTime(now.Add(-10 * time.Minute))
	m := ECSMetrics{}

	m.MemoryUsage = 100
	m.MemoryMaxUsage = 100
	m.MemoryUtilized = 100
	m.MemoryReserved = 100
	m.CPUTotalUsage = 100
	m.EphemeralStorageUtilized = 500
	m.EphemeralStorageReserved = 21000

	resource := pcommon.NewResource()
	md := convertToOTLPMetrics(taskPrefix, m, resource, timestamp, startTimestamp)
	// Task level has 26 base metrics + 2 ephemeral storage metrics = 28
	require.Equal(t, 28, md.ResourceMetrics().At(0).ScopeMetrics().Len())
	assert.Contains(t, md.ResourceMetrics().At(0).SchemaUrl(), "https://opentelemetry.io/schemas/")

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics()
	metricsByName := make(map[string]pmetric.Metric)
	for i := 0; i < scopeMetrics.Len(); i++ {
		ms := scopeMetrics.At(i).Metrics()
		for j := 0; j < ms.Len(); j++ {
			metric := ms.At(j)
			metricsByName[metric.Name()] = metric
			if metric.Type() == pmetric.MetricTypeSum {
				assert.True(t, metric.Sum().IsMonotonic())
				assert.Equal(t, pmetric.AggregationTemporalityCumulative, metric.Sum().AggregationTemporality())
				assert.Equal(t, startTimestamp, metric.Sum().DataPoints().At(0).StartTimestamp())
				assert.Equal(t, timestamp, metric.Sum().DataPoints().At(0).Timestamp())
			}
		}
	}

	utilized, ok := metricsByName[taskPrefix+attributeEphemeralStorageUtilized]
	require.True(t, ok, "expected metric %s to exist", taskPrefix+attributeEphemeralStorageUtilized)
	assert.Equal(t, unitMiB, utilized.Unit())
	assert.Equal(t, int64(500), utilized.Gauge().DataPoints().At(0).IntValue())

	reserved, ok := metricsByName[taskPrefix+attributeEphemeralStorageReserved]
	require.True(t, ok, "expected metric %s to exist", taskPrefix+attributeEphemeralStorageReserved)
	assert.Equal(t, unitMiB, reserved.Unit())
	assert.Equal(t, int64(21000), reserved.Gauge().DataPoints().At(0).IntValue())
}

func TestIntGauge(t *testing.T) {
	intValue := int64(100)
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	ilm := pmetric.NewScopeMetrics()
	appendIntGauge("cpu_utilized", "Count", intValue, timestamp, ilm)
	require.NotNil(t, ilm)
}

func TestDoubleGauge(t *testing.T) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	floatValue := 100.01

	ilm := pmetric.NewScopeMetrics()
	appendDoubleGauge("cpu_utilized", "Count", floatValue, timestamp, ilm)
	require.NotNil(t, ilm)
}

func TestIntSum(t *testing.T) {
	now := time.Now()
	timestamp := pcommon.NewTimestampFromTime(now)
	startTimestamp := pcommon.NewTimestampFromTime(now.Add(-5 * time.Minute))
	intValue := int64(100)

	ilm := pmetric.NewScopeMetrics()
	appendIntSum("cpu_utilized", "Count", intValue, timestamp, startTimestamp, ilm)
	require.NotNil(t, ilm)
	require.Equal(t, 1, ilm.Metrics().Len())

	metric := ilm.Metrics().At(0)
	assert.Equal(t, "cpu_utilized", metric.Name())
	assert.Equal(t, "Count", metric.Unit())
	assert.Equal(t, pmetric.MetricTypeSum, metric.Type())
	assert.True(t, metric.Sum().IsMonotonic())
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, metric.Sum().AggregationTemporality())
	require.Equal(t, 1, metric.Sum().DataPoints().Len())
	dp := metric.Sum().DataPoints().At(0)
	assert.Equal(t, intValue, dp.IntValue())
	assert.Equal(t, timestamp, dp.Timestamp())
	assert.Equal(t, startTimestamp, dp.StartTimestamp())
}

func TestConvertStoppedContainerDataToOTMetrics(t *testing.T) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	resource := pcommon.NewResource()
	duration := 1200000000.32132
	md := convertStoppedContainerDataToOTMetrics("container.", resource, timestamp, duration)
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())
}
