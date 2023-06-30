// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package demonset

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestDaemonsetMetrics(t *testing.T) {
	ds := testutils.NewDaemonset("1")

	actualResourceMetrics := GetMetrics(ds)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 4, len(actualResourceMetrics[0].Metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.Resource, constants.K8sType,
		map[string]string{
			"k8s.daemonset.uid":  "test-daemonset-1-uid",
			"k8s.daemonset.name": "test-daemonset-1",
			"k8s.namespace.name": "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, rm.Metrics[0], "k8s.daemonset.current_scheduled_nodes",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetricsInt(t, rm.Metrics[1], "k8s.daemonset.desired_scheduled_nodes",
		metricspb.MetricDescriptor_GAUGE_INT64, 5)

	testutils.AssertMetricsInt(t, rm.Metrics[2], "k8s.daemonset.misscheduled_nodes",
		metricspb.MetricDescriptor_GAUGE_INT64, 1)

	testutils.AssertMetricsInt(t, rm.Metrics[3], "k8s.daemonset.ready_nodes",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)
}

func TestTransform(t *testing.T) {
	originalDS := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-daemonset",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "my-app",
				},
			},
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
		Status: appsv1.DaemonSetStatus{
			CurrentNumberScheduled: 3,
			NumberReady:            3,
			DesiredNumberScheduled: 3,
			NumberMisscheduled:     0,
			Conditions: []appsv1.DaemonSetCondition{
				{
					Type:   "Available",
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	wantDS := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-daemonset",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Status: appsv1.DaemonSetStatus{
			CurrentNumberScheduled: 3,
			NumberReady:            3,
			DesiredNumberScheduled: 3,
			NumberMisscheduled:     0,
		},
	}
	assert.Equal(t, wantDS, Transform(originalDS))
}
