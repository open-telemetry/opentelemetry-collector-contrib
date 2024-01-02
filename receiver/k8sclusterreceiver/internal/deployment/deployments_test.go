// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestDeploymentMetrics(t *testing.T) {
	dep := testutils.NewDeployment("1")

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings())
	RecordMetrics(mb, dep, ts)
	m := mb.Emit()

	require.Equal(t, 1, m.ResourceMetrics().Len())
	require.Equal(t, 2, m.MetricCount())

	rm := m.ResourceMetrics().At(0)
	assert.Equal(t,
		map[string]any{
			"k8s.deployment.uid":  "test-deployment-1-uid",
			"k8s.deployment.name": "test-deployment-1",
			"k8s.namespace.name":  "test-namespace",
		},
		rm.Resource().Attributes().AsRaw(),
	)
	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sms := rm.ScopeMetrics().At(0)
	require.Equal(t, 2, sms.Metrics().Len())
	sms.Metrics().Sort(func(a, b pmetric.Metric) bool {
		return a.Name() < b.Name()
	})
	testutils.AssertMetricInt(t, sms.Metrics().At(0), "k8s.deployment.available", pmetric.MetricTypeGauge, int64(3))
	testutils.AssertMetricInt(t, sms.Metrics().At(1), "k8s.deployment.desired", pmetric.MetricTypeGauge, int64(10))
}

func TestGoldenFile(t *testing.T) {
	dep := testutils.NewDeployment("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings())
	RecordMetrics(mb, dep, ts)
	m := mb.Emit()
	expectedFile := filepath.Join("testdata", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	),
	)
}

func TestTransform(t *testing.T) {
	origDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deployment",
			UID:       "my-deployment-uid",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: appsv1.DeploymentSpec{
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
		Status: appsv1.DeploymentStatus{
			Replicas:          3,
			ReadyReplicas:     3,
			AvailableReplicas: 3,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: v1.ConditionTrue,
				},
			},
		},
	}
	wantDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deployment",
			UID:       "my-deployment-uid",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { replicas := int32(3); return &replicas }(),
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 3,
		},
	}
	assert.Equal(t, wantDeployment, Transform(origDeployment))
}
