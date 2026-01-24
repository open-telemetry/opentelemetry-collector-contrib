// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	dc := NewDataCollector(receivertest.NewNopSettings(metadata.Type), ms, metadata.DefaultMetricsBuilderConfig(), []string{"Ready"}, nil)
	m1 := dc.CollectMetricData(time.Now())

	// Verify number of resource metrics only, content is tested in other tests.
	assert.Equal(t, expectedRMs, m1.ResourceMetrics().Len())

	m2 := dc.CollectMetricData(time.Now())

	// Second scrape should be the same as the first one except for the timestamp.
	assert.NoError(t, pmetrictest.CompareMetrics(m1, m2, pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceMetricsOrder()))
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

	mbc := metadata.DefaultMetricsBuilderConfig()
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

	mbc := metadata.DefaultMetricsBuilderConfig()
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
