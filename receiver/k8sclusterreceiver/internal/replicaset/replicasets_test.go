// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicaset

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestReplicasetMetrics(t *testing.T) {
	rs := testutils.NewReplicaSet("1")

	actualResourceMetrics := GetMetrics(rs)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 2, len(actualResourceMetrics[0].Metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.Resource, constants.K8sType,
		map[string]string{
			"k8s.replicaset.uid":  "test-replicaset-1-uid",
			"k8s.replicaset.name": "test-replicaset-1",
			"k8s.namespace.name":  "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, rm.Metrics[0], "k8s.replicaset.desired",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetricsInt(t, rm.Metrics[1], "k8s.replicaset.available",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)
}

func TestTransform(t *testing.T) {
	originalRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-replicaset",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: func() *int32 { replicas := int32(3); return &replicas }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "my-app",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "my-app",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "my-container",
							Image:           "nginx:latest",
							ImagePullPolicy: v1.PullAlways,
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
									Protocol:      v1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:             3,
			FullyLabeledReplicas: 3,
			ReadyReplicas:        3,
			AvailableReplicas:    3,
		},
	}
	wantRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-replicaset",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: func() *int32 { replicas := int32(3); return &replicas }(),
		},
		Status: appsv1.ReplicaSetStatus{
			AvailableReplicas: 3,
		},
	}
	assert.Equal(t, wantRS, Transform(originalRS))
}
