// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestCalculateValue(t *testing.T) {
	value, err := calculateValue(100.0, 5.0, "add", "test_metric")
	require.NoError(t, err)
	require.Equal(t, 105.0, value)

	value, err = calculateValue(100.0, 5.0, "subtract", "test_metric")
	require.NoError(t, err)
	require.Equal(t, 95.0, value)

	value, err = calculateValue(100.0, 5.0, "multiply", "test_metric")
	require.NoError(t, err)
	require.Equal(t, 500.0, value)

	value, err = calculateValue(100.0, 5.0, "divide", "test_metric")
	require.NoError(t, err)
	require.Equal(t, 20.0, value)

	value, err = calculateValue(10.0, 200.0, "percent", "test_metric")
	require.NoError(t, err)
	require.Equal(t, 5.0, value)

	value, err = calculateValue(100.0, 0, "divide", "test_metric")
	require.Error(t, err)
	require.Equal(t, 0.0, value)

	value, err = calculateValue(100.0, 0, "percent", "test_metric")
	require.Error(t, err)
	require.Equal(t, 0.0, value)

	value, err = calculateValue(100.0, 0, "invalid", "test_metric")
	require.Error(t, err)
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
