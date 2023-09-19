// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func AssertMetricInt(t testing.TB, m pmetric.Metric, expectedMetric string, expectedType pmetric.MetricType, expectedValue any) {
	dps := assertMetric(t, m, expectedMetric, expectedType)
	require.EqualValues(t, expectedValue, dps.At(0).IntValue(), "mismatching metric values")
}

func assertMetric(t testing.TB, m pmetric.Metric, expectedMetric string, expectedType pmetric.MetricType) pmetric.NumberDataPointSlice {
	require.Equal(t, expectedMetric, m.Name(), "mismatching metric names")
	require.NotEmpty(t, m.Description(), "empty description on metric")
	require.Equal(t, expectedType, m.Type(), "mismatching metric types")
	var dps pmetric.NumberDataPointSlice
	//exhaustive:enforce
	switch expectedType {
	case pmetric.MetricTypeGauge:
		dps = m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = m.Sum().DataPoints()
	case pmetric.MetricTypeHistogram:
		require.Fail(t, "unsupported")
	case pmetric.MetricTypeExponentialHistogram:
		require.Fail(t, "unsupported")
	case pmetric.MetricTypeSummary:
		require.Fail(t, "unsupported")
	case pmetric.MetricTypeEmpty:
		require.Fail(t, "unsupported")
	}
	require.Equal(t, 1, dps.Len())
	return dps
}
