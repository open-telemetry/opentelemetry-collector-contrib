// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/require"
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
