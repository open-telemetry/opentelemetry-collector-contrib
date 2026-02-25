// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestTransformObject(t *testing.T) {
	i := 1
	intPtr := &i
	tests := []struct {
		name   string
		object any
		want   any
		same   bool
	}{
		{
			name: "pod",
			object: testutils.NewPodWithContainer(
				"1",
				testutils.NewPodSpecWithContainer("container-name"),
				testutils.NewPodStatusWithContainer("container-name", "container-id"),
			),
			want: func() *corev1.Pod {
				pod := testutils.NewPodWithContainer(
					"1",
					testutils.NewPodSpecWithContainer("container-name"),
					testutils.NewPodStatusWithContainer("container-name", "container-id"),
				)
				pod.Spec.Containers[0].Image = ""
				return pod
			}(),
			same: false,
		},
		{
			name:   "node",
			object: testutils.NewNode("1"),
			want:   testutils.NewNode("1"),
			same:   false,
		},
		{
			name:   "replicaset",
			object: testutils.NewReplicaSet("1"),
			want:   testutils.NewReplicaSet("1"),
			same:   false,
		},
		{
			name:   "job",
			object: testutils.NewJob("1"),
			want:   testutils.NewJob("1"),
			same:   false,
		},
		{
			name:   "deployment",
			object: testutils.NewDeployment("1"),
			want:   testutils.NewDeployment("1"),
			same:   false,
		},
		{
			name:   "daemonset",
			object: testutils.NewDaemonset("1"),
			want:   testutils.NewDaemonset("1"),
			same:   false,
		},
		{
			name: "statefulset",
			object: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(3); return &i }(),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "my-app",
							},
						},
					},
				},
			},
			want: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(3); return &i }(),
				},
			},
			same: false,
		},
		{
			name: "service",
			object: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "my-app",
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			},
			want: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "my-app",
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			},
			same: false,
		},
		{
			name:   "endpointslice",
			object: testutils.NewEndpointSlice("1"),
			want:   testutils.NewEndpointSlice("1"),
			same:   false,
		},
		{
			// This is a case where we don't transform the object.
			name:   "hpa",
			object: testutils.NewHPA("1"),
			want:   testutils.NewHPA("1"),
			same:   true,
		},
		{
			name:   "invalid_type",
			object: intPtr,
			want:   intPtr,
			same:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformObject(tt.object)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if tt.same {
				assert.Same(t, tt.object, got)
			} else {
				assert.NotSame(t, tt.object, got)
			}
		})
	}
}
