// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestCalculateValue(t *testing.T) {
	value := calculateValue(100.0, 5.0, "add", zap.NewNop(), "test_metric")
	require.Equal(t, 105.0, value)

	value = calculateValue(100.0, 5.0, "subtract", zap.NewNop(), "test_metric")
	require.Equal(t, 95.0, value)

	value = calculateValue(100.0, 5.0, "multiply", zap.NewNop(), "test_metric")
	require.Equal(t, 500.0, value)

	value = calculateValue(100.0, 5.0, "divide", zap.NewNop(), "test_metric")
	require.Equal(t, 20.0, value)

	value = calculateValue(10.0, 200.0, "percent", zap.NewNop(), "test_metric")
	require.Equal(t, 5.0, value)

	value = calculateValue(100.0, 0, "divide", zap.NewNop(), "test_metric")
	require.Equal(t, 0.0, value)

	value = calculateValue(100.0, 0, "percent", zap.NewNop(), "test_metric")
	require.Equal(t, 0.0, value)

	value = calculateValue(100.0, 0, "invalid", zap.NewNop(), "test_metric")
	require.Equal(t, 0.0, value)
}

func TestGetMetricValueWithNoDataPoint(t *testing.T) {
	md := pmetric.NewMetrics()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	m := ms.AppendEmpty()
	m.SetName("metric_1")
	m.SetEmptyGauge()

	value := getMetricValue(md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0))
	require.Equal(t, 0.0, value)
}
