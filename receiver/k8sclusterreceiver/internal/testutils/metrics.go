// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func AssertResource(t *testing.T, actualResource *resourcepb.Resource,
	expectedType string, expectedLabels map[string]string) {
	require.Equal(t,
		expectedType,
		actualResource.Type,
		"mismatching resource types",
	)

	require.Equal(t,
		expectedLabels,
		actualResource.Labels,
		"mismatching resource labels",
	)
}

func AssertMetricInt(t testing.TB, m pmetric.Metric, expectedMetric string, expectedType pmetric.MetricType, expectedValue any) {
	dps := assertMetric(t, m, expectedMetric, expectedType)
	require.EqualValues(t, expectedValue, dps.At(0).IntValue(), "mismatching metric values")
}

func assertMetric(t testing.TB, m pmetric.Metric, expectedMetric string, expectedType pmetric.MetricType) pmetric.NumberDataPointSlice {
	require.Equal(t, expectedMetric, m.Name(), "mismatching metric names")
	require.NotEmpty(t, m.Description(), "empty description on metric")
	require.Equal(t, expectedType, m.Type(), "mismatching metric types")
	var dps pmetric.NumberDataPointSlice
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

func AssertMetricsWithLabels(t *testing.T, actualMetric *metricspb.Metric,
	expectedMetric string, expectedType metricspb.MetricDescriptor_Type,
	expectedLabels map[string]string, expectedValue int64) {

	require.Equal(t,
		len(expectedLabels),
		len(actualMetric.MetricDescriptor.LabelKeys),
		"mismatching number of labels",
	)

	require.Equal(t,
		expectedLabels,
		getLabelsMap(actualMetric),
		"mismatching labels",
	)

	AssertMetricsInt(t, actualMetric, expectedMetric, expectedType, expectedValue)
}

func AssertMetricsInt(t *testing.T, actualMetric *metricspb.Metric, expectedMetric string,
	expectedType metricspb.MetricDescriptor_Type, expectedValue int64) {
	assertMetricsBase(t, actualMetric, expectedMetric, expectedType)

	require.Equal(t,
		expectedValue,
		actualMetric.Timeseries[0].Points[0].GetInt64Value(),
		"mismatching metric values",
	)
}

func AssertMetricsDouble(t *testing.T, actualMetric *metricspb.Metric, expectedMetric string,
	expectedType metricspb.MetricDescriptor_Type, expectedValue float64) {
	assertMetricsBase(t, actualMetric, expectedMetric, expectedType)

	require.Equal(t,
		expectedValue,
		actualMetric.Timeseries[0].Points[0].GetDoubleValue(),
		"mismatching metric values",
	)
}

func assertMetricsBase(t *testing.T, actualMetric *metricspb.Metric, expectedMetric string,
	expectedType metricspb.MetricDescriptor_Type) {

	require.Equal(t,
		expectedMetric,
		actualMetric.MetricDescriptor.Name,
		"mismatching metric names",
	)

	require.NotEmpty(t,
		actualMetric.MetricDescriptor.Description,
		"empty description on metric",
	)

	require.Equal(t,
		expectedType,
		actualMetric.MetricDescriptor.Type,
		"mismatching metric types",
	)
}

// getLabelsMap returns a map of labels.
func getLabelsMap(m *metricspb.Metric) map[string]string {
	out := map[string]string{}
	for i, k := range m.MetricDescriptor.LabelKeys {
		out[k.Key] = m.Timeseries[0].LabelValues[i].Value
	}

	return out
}
