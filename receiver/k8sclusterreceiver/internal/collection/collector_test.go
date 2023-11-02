// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestCollectMetricData(t *testing.T) {
	ms := metadata.NewStore()
	var expectedRMs int

	ms.Setup(gvk.Pod, &testutils.MockStore{
		Cache: map[string]any{
			"pod1-uid": testutils.NewPodWithContainer(
				"1",
				testutils.NewPodSpecWithContainer("container-name"),
				testutils.NewPodStatusWithContainer("container-name", "container-id"),
			),
		},
	})
	expectedRMs += 2 // 1 for pod, 1 for container

	ms.Setup(gvk.Node, &testutils.MockStore{
		Cache: map[string]any{
			"node1-uid": testutils.NewNode("1"),
			"node2-uid": testutils.NewNode("2"),
		},
	})
	expectedRMs += 2

	ms.Setup(gvk.Namespace, &testutils.MockStore{
		Cache: map[string]any{
			"namespace1-uid": testutils.NewNamespace("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.ReplicationController, &testutils.MockStore{
		Cache: map[string]any{
			"replicationcontroller1-uid": testutils.NewReplicationController("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.ResourceQuota, &testutils.MockStore{
		Cache: map[string]any{
			"resourcequota1-uid": testutils.NewResourceQuota("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.Deployment, &testutils.MockStore{
		Cache: map[string]any{
			"deployment1-uid": testutils.NewDeployment("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.ReplicaSet, &testutils.MockStore{
		Cache: map[string]any{
			"replicaset1-uid": testutils.NewReplicaSet("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.DaemonSet, &testutils.MockStore{
		Cache: map[string]any{
			"daemonset1-uid": testutils.NewDaemonset("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.StatefulSet, &testutils.MockStore{
		Cache: map[string]any{
			"statefulset1-uid": testutils.NewStatefulset("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.Job, &testutils.MockStore{
		Cache: map[string]any{
			"job1-uid": testutils.NewJob("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.CronJob, &testutils.MockStore{
		Cache: map[string]any{
			"cronjob1-uid": testutils.NewCronJob("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.HorizontalPodAutoscaler, &testutils.MockStore{
		Cache: map[string]any{
			"horizontalpodautoscaler1-uid": testutils.NewHPA("1"),
		},
	})
	expectedRMs++

	dc := NewDataCollector(receivertest.NewNopCreateSettings(), ms, metadata.DefaultMetricsBuilderConfig(), []string{"Ready"}, nil)
	m1 := dc.CollectMetricData(time.Now())

	// Verify number of resource metrics only, content is tested in other tests.
	assert.Equal(t, expectedRMs, m1.ResourceMetrics().Len())

	m2 := dc.CollectMetricData(time.Now())

	// Second scrape should be the same as the first one except for the timestamp.
	assert.NoError(t, pmetrictest.CompareMetrics(m1, m2, pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceMetricsOrder()))
}

func TestCollectMetricDataWithNodeMetadata(t *testing.T) {
	ms := metadata.NewStore()
	var expectedRMs int

	node1 := testutils.NewNode("1")
	node1.Annotations = map[string]string{
		"annotation-1": "annotation-1",
		"annotation-2": "annotation-2",
	}
	node1.Labels = map[string]string{
		"label-1": "label-1",
		"label-2": "label-2",
	}

	node2 := testutils.NewNode("2")

	ms.Setup(gvk.Node, &testutils.MockStore{
		Cache: map[string]interface{}{
			node1.Name: node1,
			node2.Name: node2,
		},
	})
	expectedRMs += 2

	pod1 := testutils.NewPodWithContainer(
		"1",
		testutils.NewPodSpecWithContainer("container-name"),
		testutils.NewPodStatusWithContainer("container-name", "container-id"),
	)
	pod1.Spec.NodeName = node1.Name

	ms.Setup(gvk.Pod, &testutils.MockStore{
		Cache: map[string]interface{}{
			pod1.Name: pod1,
		},
	})
	expectedRMs += 2 // 1 for pod, 1 for container

	tests := []struct {
		name         string
		config       metadata.MetricsBuilderConfig
		expectedFile string
	}{
		{
			name:         "collect with default config",
			config:       metadata.DefaultMetricsBuilderConfig(),
			expectedFile: filepath.Join("testdata", "expected-without-node-metadata.yaml"),
		},
		{
			name: "collect with node metadata enabled",
			config: func() metadata.MetricsBuilderConfig {
				c := metadata.DefaultMetricsBuilderConfig()
				c.ResourceAttributes.K8sNodeLabels.Enabled = true
				c.ResourceAttributes.K8sNodeAnnotations.Enabled = true
				return c
			}(),
			expectedFile: filepath.Join("testdata", "expected-node-metadata.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := NewDataCollector(receivertest.NewNopCreateSettings(), ms, tt.config, []string{"Ready"}, nil)
			m1 := dc.CollectMetricData(time.Now())
			assert.Equal(t, expectedRMs, m1.ResourceMetrics().Len())

			expected, err := golden.ReadMetrics(tt.expectedFile)
			require.NoError(t, err)

			require.NoError(t,
				pmetrictest.CompareMetrics(expected, m1,
					pmetrictest.IgnoreTimestamp(),
					pmetrictest.IgnoreStartTimestamp(),
					pmetrictest.IgnoreResourceMetricsOrder(),
					pmetrictest.IgnoreMetricsOrder(),
					pmetrictest.IgnoreScopeMetricsOrder(),
				))
		})
	}

}
