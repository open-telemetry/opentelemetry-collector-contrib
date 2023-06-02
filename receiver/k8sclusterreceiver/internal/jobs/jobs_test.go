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

package jobs

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestJobMetrics(t *testing.T) {
	j := testutils.NewJob("1")

	actualResourceMetrics := GetMetrics(j)

	require.Equal(t, 1, len(actualResourceMetrics))

	require.Equal(t, 5, len(actualResourceMetrics[0].Metrics))
	testutils.AssertResource(t, actualResourceMetrics[0].Resource, constants.K8sType,
		map[string]string{
			"k8s.job.uid":        "test-job-1-uid",
			"k8s.job.name":       "test-job-1",
			"k8s.namespace.name": "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[0], "k8s.job.active_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[1], "k8s.job.failed_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 0)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[2], "k8s.job.successful_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[3], "k8s.job.desired_successful_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 10)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[4], "k8s.job.max_parallel_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)

	// Test with nil values.
	j.Spec.Completions = nil
	j.Spec.Parallelism = nil
	actualResourceMetrics = GetMetrics(j)
	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 3, len(actualResourceMetrics[0].Metrics))

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[0], "k8s.job.active_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[1], "k8s.job.failed_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 0)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[2], "k8s.job.successful_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)
}
