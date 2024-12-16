// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func AssertMetricInt(tb testing.TB, m pmetric.Metric, expectedMetric string, expectedType pmetric.MetricType, expectedValue any) {
	dps := assertMetric(tb, m, expectedMetric, expectedType)
	require.EqualValues(tb, expectedValue, dps.At(0).IntValue(), "mismatching metric values")
}

func assertMetric(tb testing.TB, m pmetric.Metric, expectedMetric string, expectedType pmetric.MetricType) pmetric.NumberDataPointSlice {
	require.Equal(tb, expectedMetric, m.Name(), "mismatching metric names")
	require.NotEmpty(tb, m.Description(), "empty description on metric")
	require.Equal(tb, expectedType, m.Type(), "mismatching metric types")
	var dps pmetric.NumberDataPointSlice
	//exhaustive:enforce
	switch expectedType {
	case pmetric.MetricTypeGauge:
		dps = m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = m.Sum().DataPoints()
	case pmetric.MetricTypeHistogram:
		require.Fail(tb, "unsupported")
	case pmetric.MetricTypeExponentialHistogram:
		require.Fail(tb, "unsupported")
	case pmetric.MetricTypeSummary:
		require.Fail(tb, "unsupported")
	case pmetric.MetricTypeEmpty:
		require.Fail(tb, "unsupported")
	}
	require.Equal(tb, 1, dps.Len())
	return dps
}
