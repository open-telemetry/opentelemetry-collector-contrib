// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestCollectMetricData(t *testing.T) {
	ms := metadata.NewStore()
	var expectedRMs int

	ms.Setup(gvk.Pod, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"pod1-uid": testutils.NewPodWithContainer(
				"1",
				testutils.NewPodSpecWithContainer("container-name"),
				testutils.NewPodStatusWithContainer("container-name", "container-id"),
			),
		},
	})
	expectedRMs += 2 // 1 for pod, 1 for container

	ms.Setup(gvk.Node, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"node1-uid": testutils.NewNode("1"),
			"node2-uid": testutils.NewNode("2"),
		},
	})
	expectedRMs += 2

	ms.Setup(gvk.Namespace, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"namespace1-uid": testutils.NewNamespace("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.ReplicationController, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"replicationcontroller1-uid": testutils.NewReplicationController("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.ResourceQuota, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"resourcequota1-uid": testutils.NewResourceQuota("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.Deployment, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"deployment1-uid": testutils.NewDeployment("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.ReplicaSet, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"replicaset1-uid": testutils.NewReplicaSet("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.DaemonSet, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"daemonset1-uid": testutils.NewDaemonset("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.StatefulSet, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"statefulset1-uid": testutils.NewStatefulset("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.Job, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"job1-uid": testutils.NewJob("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.CronJob, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"cronjob1-uid": testutils.NewCronJob("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.HorizontalPodAutoscaler, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"horizontalpodautoscaler1-uid": testutils.NewHPA("1"),
		},
	})
	expectedRMs++

	ms.Setup(gvk.Service, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"service1-uid": testutils.NewService("1"),
		},
	})

	ms.Setup(gvk.EndpointSlice, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"endpointslice1-uid": testutils.NewEndpointSlice("1"),
		},
	})

	dc := NewDataCollector(receivertest.NewNopSettings(metadata.Type), ms, metadata.NewDefaultMetricsBuilderConfig(), []string{"Ready"}, nil)
	m1 := dc.CollectMetricData(time.Now())

	// Verify number of resource metrics only, content is tested in other tests.
	assert.Equal(t, expectedRMs, m1.ResourceMetrics().Len())

	m2 := dc.CollectMetricData(time.Now())

	// Second scrape should be the same as the first one except for the timestamp.
	assert.NoError(t, pmetrictest.CompareMetrics(m1, m2, pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceMetricsOrder()))
}

func TestCollectMetricDataWithClusterUID(t *testing.T) {
	dc := NewDataCollector(receivertest.NewNopSettings(metadata.Type), newStoreWithPodAndNode(),
		metadata.NewDefaultMetricsBuilderConfig(), []string{"Ready"}, nil)
	dc.SetClusterUID("cluster1-uid")

	// Every resource, including the ones built by the custom metrics path, carries the cluster UID.
	rms := dc.CollectMetricData(time.Now()).ResourceMetrics()
	require.Positive(t, rms.Len())
	for i := 0; i < rms.Len(); i++ {
		clusterUID, ok := rms.At(i).Resource().Attributes().Get("k8s.cluster.uid")
		require.True(t, ok, "k8s.cluster.uid is missing from resource %d", i)
		assert.Equal(t, "cluster1-uid", clusterUID.Str())
	}
}

func TestCollectMetricDataWithoutClusterUID(t *testing.T) {
	dc := NewDataCollector(receivertest.NewNopSettings(metadata.Type), newStoreWithPodAndNode(),
		metadata.NewDefaultMetricsBuilderConfig(), []string{"Ready"}, nil)

	// Without a cluster UID, the resource attribute is left off.
	rms := dc.CollectMetricData(time.Now()).ResourceMetrics()
	require.Positive(t, rms.Len())
	for i := 0; i < rms.Len(); i++ {
		_, ok := rms.At(i).Resource().Attributes().Get("k8s.cluster.uid")
		assert.False(t, ok, "k8s.cluster.uid should not be set when the cluster UID is unknown")
	}
}

// With leader election in place the receiver starts over, and thus might set the cluster UID again,
// on every lease acquisition, while the collection of the previous term may still be running. Run
// both concurrently so that the race detector covers that overlap.
func TestSetClusterUIDConcurrentlyWithCollectMetricData(t *testing.T) {
	dc := NewDataCollector(receivertest.NewNopSettings(metadata.Type), newStoreWithPodAndNode(),
		metadata.NewDefaultMetricsBuilderConfig(), []string{"Ready"}, nil)
	dc.SetClusterUID("cluster1-uid")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			dc.SetClusterUID("cluster1-uid")
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			dc.CollectMetricData(time.Now())
		}
	}()
	wg.Wait()

	rms := dc.CollectMetricData(time.Now()).ResourceMetrics()
	require.Positive(t, rms.Len())
	for i := 0; i < rms.Len(); i++ {
		clusterUID, ok := rms.At(i).Resource().Attributes().Get("k8s.cluster.uid")
		require.True(t, ok, "k8s.cluster.uid is missing from resource %d", i)
		assert.Equal(t, "cluster1-uid", clusterUID.Str())
	}
}

// newStoreWithPodAndNode returns a metadata store holding one pod and one node. Nodes are reported
// through the custom metrics path, which builds its own resource metrics.
func newStoreWithPodAndNode() *metadata.Store {
	ms := metadata.NewStore()

	ms.Setup(gvk.Pod, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"pod1-uid": testutils.NewPodWithContainer(
				"1",
				testutils.NewPodSpecWithContainer("container-name"),
				testutils.NewPodStatusWithContainer("container-name", "container-id"),
			),
		},
	})

	ms.Setup(gvk.Node, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"node1-uid": testutils.NewNode("1"),
		},
	})

	return ms
}

func TestCollectServiceMetrics(t *testing.T) {
	ms := metadata.NewStore()

	ms.Setup(gvk.Service, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"service1-uid": testutils.NewService("1"),
		},
	})

	ms.Setup(gvk.EndpointSlice, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"endpointslice1-uid": testutils.NewEndpointSlice("1"),
		},
	})

	mbc := metadata.NewDefaultMetricsBuilderConfig()
	mbc.Metrics.K8sServiceEndpointCount.Enabled = true
	dc := NewDataCollector(receivertest.NewNopSettings(metadata.Type), ms, mbc, nil, nil)
	m := dc.CollectMetricData(time.Now())

	foundEndpointCount := false
	foundLBIngressCount := false

	rm := m.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		sm := rm.At(i).ScopeMetrics()
		for j := 0; j < sm.Len(); j++ {
			ms := sm.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				if metric.Name() == "k8s.service.endpoint.count" {
					foundEndpointCount = true
					// Verify attributes
					dps := metric.Gauge().DataPoints()
					assert.Positive(t, dps.Len())
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						conditionAttr, ok := dp.Attributes().Get("k8s.service.endpoint.condition")
						assert.True(t, ok)
						assert.Contains(t, []string{"ready", "serving", "terminating"}, conditionAttr.Str())

						addressTypeAttr, ok := dp.Attributes().Get("k8s.service.endpoint.address_type")
						assert.True(t, ok)
						assert.Equal(t, "IPv4", addressTypeAttr.Str(), "AddressType should be preserved from EndpointSlice")
					}
				}
				if metric.Name() == "k8s.service.load_balancer.ingress.count" {
					// ClusterIP service shouldn't emit this metric
					foundLBIngressCount = true
				}
			}
		}
	}

	assert.True(t, foundEndpointCount, "Expected k8s.service.endpoint.count metric")
	assert.False(t, foundLBIngressCount, "Did not expect k8s.service.load_balancer.ingress.count metric for ClusterIP service")
}

func TestCollectLoadBalancerServiceMetrics(t *testing.T) {
	ms := metadata.NewStore()

	ms.Setup(gvk.Service, metadata.ClusterWideInformerKey, &testutils.MockStore{
		Cache: map[string]any{
			"lb-service1-uid": testutils.NewLoadBalancerService("1"),
		},
	})

	mbc := metadata.NewDefaultMetricsBuilderConfig()
	mbc.Metrics.K8sServiceLoadBalancerIngressCount.Enabled = true
	dc := NewDataCollector(receivertest.NewNopSettings(metadata.Type), ms, mbc, nil, nil)
	m := dc.CollectMetricData(time.Now())

	foundLBIngressCount := false

	rm := m.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		sm := rm.At(i).ScopeMetrics()
		for j := 0; j < sm.Len(); j++ {
			ms := sm.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				if metric.Name() == "k8s.service.load_balancer.ingress.count" {
					foundLBIngressCount = true
					assert.Equal(t, int64(1), metric.Gauge().DataPoints().At(0).IntValue())
				}
			}
		}
	}

	assert.True(t, foundLBIngressCount, "Expected k8s.service.load_balancer.ingress.count metric for LoadBalancer service")
}
