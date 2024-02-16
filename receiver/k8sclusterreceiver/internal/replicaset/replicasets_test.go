// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicaset

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestReplicasetMetrics(t *testing.T) {
	rs := testutils.NewReplicaSet("1")

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings())
	RecordMetrics(mb, rs, ts)
	m := mb.Emit()
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
