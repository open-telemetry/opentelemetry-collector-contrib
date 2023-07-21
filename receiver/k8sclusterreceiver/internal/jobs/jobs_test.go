// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestJobMetrics(t *testing.T) {
	j := testutils.NewJob("1")

	m := GetMetrics(receivertest.NewNopCreateSettings(), j)

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
	),
	)
	// Test with nil values.
	j.Spec.Completions = nil
	j.Spec.Parallelism = nil
	m = GetMetrics(receivertest.NewNopCreateSettings(), j)
	expected, err = golden.ReadMetrics(filepath.Join("testdata", "expected_empty.yaml"))
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
	),
	)
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
