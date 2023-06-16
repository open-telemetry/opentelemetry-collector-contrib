// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestTransformObject(t *testing.T) {
	i := 1
	intPtr := &i
	tests := []struct {
		name   string
		object interface{}
		want   interface{}
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
				pod.Status.ContainerStatuses[0].State = corev1.ContainerState{}
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
