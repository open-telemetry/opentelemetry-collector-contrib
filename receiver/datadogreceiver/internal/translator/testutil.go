// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	scopeName              string = "otelcol/datadogreceiver"
	aggregationTemporality        = pmetric.AggregationTemporalityDelta
)

func createMetricsTranslator() *MetricsTranslator {
	mt := NewMetricsTranslator(component.BuildInfo{
		Command:     "otelcol",
		Description: "OpenTelemetry Collector",
		Version:     "latest",
	})
	return mt
}

func requireResourceAttributes(t *testing.T, attrs, expectedAttrs pcommon.Map) {
	expectedAttrs.Range(func(k string, _ pcommon.Value) bool {
		ev, _ := expectedAttrs.Get(k)
		av, ok := attrs.Get(k)
		require.True(t, ok)
		require.Equal(t, ev, av)
		return true
	})
}

// nolint:unparam
func requireScopeMetrics(t *testing.T, result pmetric.Metrics, expectedScopeMetricsLen, expectedMetricsLen int) {
	require.Equal(t, expectedScopeMetricsLen, result.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, expectedMetricsLen, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
}

func requireScope(t *testing.T, result pmetric.Metrics, expectedAttrs pcommon.Map, expectedVersion string) {
	require.Equal(t, scopeName, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
	require.Equal(t, expectedVersion, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Version())
	require.Equal(t, expectedAttrs, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes())
}

func requireMetricAndDataPointCounts(t *testing.T, result pmetric.Metrics, expectedMetricCount, expectedDpCount int) {
	require.Equal(t, expectedMetricCount, result.MetricCount())
	require.Equal(t, expectedDpCount, result.DataPointCount())
}

func requireSum(t *testing.T, metric pmetric.Metric, expectedName string, expectedDpsLen int) {
	require.Equal(t, expectedName, metric.Name())
	require.Equal(t, pmetric.MetricTypeSum, metric.Type())
	require.Equal(t, aggregationTemporality, metric.Sum().AggregationTemporality())
	require.Equal(t, expectedDpsLen, metric.Sum().DataPoints().Len())
}

func requireGauge(t *testing.T, metric pmetric.Metric, expectedName string, expectedDpsLen int) {
	require.Equal(t, expectedName, metric.Name())
	require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	require.Equal(t, expectedDpsLen, metric.Gauge().DataPoints().Len())
}

func requireDp(t *testing.T, dp pmetric.NumberDataPoint, expectedAttrs pcommon.Map, expectedTime int64, expectedValue float64) {
	require.Equal(t, expectedTime, dp.Timestamp().AsTime().Unix())
	require.Equal(t, expectedValue, dp.DoubleValue())
	require.Equal(t, expectedAttrs, dp.Attributes())
}
