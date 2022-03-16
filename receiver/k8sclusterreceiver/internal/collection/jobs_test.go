// Copyright 2020 OpenTelemetry Authors
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

package collection

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestJobMetrics(t *testing.T) {
	j := newJob("1")

	actualResourceMetrics := getMetricsForJob(j)

	require.Equal(t, 1, len(actualResourceMetrics))

	require.Equal(t, 5, len(actualResourceMetrics[0].metrics))
	testutils.AssertResource(t, actualResourceMetrics[0].resource, k8sType,
		map[string]string{
			"k8s.job.uid":        "test-job-1-uid",
			"k8s.job.name":       "test-job-1",
			"k8s.namespace.name": "test-namespace",
			"k8s.cluster.name":   "test-cluster",
		},
	)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].metrics[0], "k8s.job.active_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].metrics[1], "k8s.job.failed_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 0)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].metrics[2], "k8s.job.successful_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].metrics[3], "k8s.job.desired_successful_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 10)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].metrics[4], "k8s.job.max_parallel_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)

	// Test with nil values.
	j.Spec.Completions = nil
	j.Spec.Parallelism = nil
	actualResourceMetrics = getMetricsForJob(j)
	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 3, len(actualResourceMetrics[0].metrics))

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].metrics[0], "k8s.job.active_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].metrics[1], "k8s.job.failed_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 0)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].metrics[2], "k8s.job.successful_pods",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)
}

func newJob(id string) *batchv1.Job {
	p := int32(2)
	c := int32(10)
	return &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:        "test-job-" + id,
			Namespace:   "test-namespace",
			UID:         types.UID("test-job-" + id + "-uid"),
			ClusterName: "test-cluster",
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: &p,
			Completions: &c,
		},
		Status: batchv1.JobStatus{
			Active:    2,
			Succeeded: 3,
			Failed:    0,
		},
	}
}
