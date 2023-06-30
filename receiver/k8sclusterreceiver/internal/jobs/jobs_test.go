// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func TestTransform(t *testing.T) {
	originalJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-job",
			Namespace: "default",
			UID:       "my-job-uid",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: batchv1.JobSpec{
			Completions: func() *int32 { completions := int32(1); return &completions }(),
			Parallelism: func() *int32 { parallelism := int32(1); return &parallelism }(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "my-app",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "my-container",
							Image:           "busybox",
							Command:         []string{"echo", "Hello, World!"},
							ImagePullPolicy: corev1.PullAlways,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
		Status: batchv1.JobStatus{
			Active:    1,
			Succeeded: 2,
			Failed:    3,
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	wantJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-job",
			Namespace: "default",
			UID:       "my-job-uid",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: batchv1.JobSpec{
			Completions: func() *int32 { completions := int32(1); return &completions }(),
			Parallelism: func() *int32 { parallelism := int32(1); return &parallelism }(),
		},
		Status: batchv1.JobStatus{
			Active:    1,
			Succeeded: 2,
			Failed:    3,
		},
	}
	assert.Equal(t, wantJob, Transform(originalJob))
}
