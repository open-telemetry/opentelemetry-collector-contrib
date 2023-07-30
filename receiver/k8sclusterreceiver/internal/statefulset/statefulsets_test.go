// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statefulset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func TestStatefulsetMetrics(t *testing.T) {
	ss := newStatefulset("1")

	actualResourceMetrics := GetMetrics(receivertest.NewNopCreateSettings(), ss)

	require.Equal(t, 1, actualResourceMetrics.ResourceMetrics().Len())
	require.Equal(t, 4, actualResourceMetrics.MetricCount())

	rm := actualResourceMetrics.ResourceMetrics().At(0)
	assert.Equal(t,
		map[string]interface{}{
			"k8s.statefulset.uid":     "test-statefulset-1-uid",
			"k8s.statefulset.name":    "test-statefulset-1",
			"k8s.namespace.name":      "test-namespace",
			"opencensus.resourcetype": "k8s",
		}, rm.Resource().Attributes().AsRaw(),
	)

	m1 := rm.ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "k8s.statefulset.current_pods", m1.Name())

	m2 := rm.ScopeMetrics().At(0).Metrics().At(1)
	assert.Equal(t, "k8s.statefulset.desired_pods", m2.Name())

	m3 := rm.ScopeMetrics().At(0).Metrics().At(2)
	assert.Equal(t, "k8s.statefulset.ready_pods", m3.Name())
	m4 := rm.ScopeMetrics().At(0).Metrics().At(3)
	assert.Equal(t, "k8s.statefulset.updated_pods", m4.Name())
}

func TestStatefulsetMetadata(t *testing.T) {
	ss := newStatefulset("1")

	actualMetadata := GetMetadata(ss)

	require.Equal(t, 1, len(actualMetadata))

	require.Equal(t,
		metadata.KubernetesMetadata{
			EntityType:    "k8s.statefulset",
			ResourceIDKey: "k8s.statefulset.uid",
			ResourceID:    "test-statefulset-1-uid",
			Metadata: map[string]string{
				"k8s.workload.name":              "test-statefulset-1",
				"k8s.workload.kind":              "StatefulSet",
				"statefulset.creation_timestamp": "0001-01-01T00:00:00Z",
				"foo":                            "bar",
				"foo1":                           "",
				"current_revision":               "current_revision",
				"update_revision":                "update_revision",
			},
		},
		*actualMetadata["test-statefulset-1-uid"],
	)
}

func newStatefulset(id string) *appsv1.StatefulSet {
	desired := int32(10)
	return &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-statefulset-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-statefulset-" + id + "-uid"),
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &desired,
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas:   7,
			CurrentReplicas: 5,
			UpdatedReplicas: 3,
			CurrentRevision: "current_revision",
			UpdateRevision:  "update_revision",
		},
	}
}

func TestTransform(t *testing.T) {
	orig := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "my-statefulset",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: func() *int32 { i := int32(3); return &i }(),
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "my-app",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"app": "my-app",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "my-container",
							Image:           "nginx:latest",
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:        3,
			ReadyReplicas:   3,
			CurrentReplicas: 3,
			UpdatedReplicas: 3,
			Conditions: []appsv1.StatefulSetCondition{
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	want := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "my-statefulset",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: func() *int32 { i := int32(3); return &i }(),
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas:   3,
			CurrentReplicas: 3,
			UpdatedReplicas: 3,
		},
	}
	assert.Equal(t, want, Transform(orig))
}
