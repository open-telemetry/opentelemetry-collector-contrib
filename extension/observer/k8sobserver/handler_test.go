// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func newTestHandler() *handler {
	h := &handler{
		idNamespace: "test-1",
		endpoints:   &sync.Map{},
		logger:      zap.NewNop(),
	}
	return h
}

func TestPodEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(podWithNamedPorts, true)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/pod-2-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
		{
			ID:     "test-1/pod-2-UID/container-2",
			Target: "1.2.3.4",
			Details: &observer.PodContainer{
				Name:        "container-2",
				Image:       "container-image-2",
				ContainerID: "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
		{
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https",
				Pod: observer.Pod{
					Namespace: "default",
					UID:       "pod-2-UID",
					Name:      "pod-2",
					Labels:    map[string]string{"env": "prod"},
				},
				Port:      443,
				Transport: observer.ProtocolTCP,
			},
		},
	}, th.ListEndpoints())
}

func TestPodEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(podWithNamedPorts, true)
	th.OnDelete(podWithNamedPorts)
	assert.Empty(t, th.ListEndpoints())
}

func TestPodEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.OnUpdate(podWithNamedPorts, podWithNamedPorts)
	require.Empty(t, th.ListEndpoints())

	// Labels changed.
	changedLabels := podWithNamedPorts.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.OnUpdate(podWithNamedPorts, changedLabels)

	endpoints := th.ListEndpoints()
	require.ElementsMatch(t,
		[]observer.EndpointID{"test-1/pod-2-UID", "test-1/pod-2-UID/container-2", "test-1/pod-2-UID/https(443)"},
		[]observer.EndpointID{endpoints[0].ID, endpoints[1].ID, endpoints[2].ID},
	)

	// Running state changed, one added and one removed.
	updatedPod := podWithNamedPorts.DeepCopy()
	updatedPod.Labels["updated-label"] = "true"
	th.OnUpdate(podWithNamedPorts, updatedPod)
	require.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/pod-2-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod", "updated-label": "true"},
			},
		},
		{
			ID:     "test-1/pod-2-UID/container-2",
			Target: "1.2.3.4",
			Details: &observer.PodContainer{
				Name:        "container-2",
				Image:       "container-image-2",
				ContainerID: "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod", "updated-label": "true"},
				},
			},
		},
		{
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https", Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod", "updated-label": "true"},
				},
				Port:      443,
				Transport: observer.ProtocolTCP,
			},
		},
	}, th.ListEndpoints())
}

func TestServiceEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(serviceWithClusterIP, true)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/service-1-UID",
			Target: "service-1.default.svc.cluster.local",
			Details: &observer.K8sService{
				Name:        "service-1",
				Namespace:   "default",
				UID:         "service-1-UID",
				Labels:      map[string]string{"env": "prod"},
				ServiceType: "ClusterIP",
				ClusterIP:   "1.2.3.4",
			},
		},
	}, th.ListEndpoints())
}

func TestServiceEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(serviceWithClusterIP, true)
	th.OnDelete(serviceWithClusterIP)
	assert.Empty(t, th.ListEndpoints())
}

func TestServiceEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.OnUpdate(serviceWithClusterIP, serviceWithClusterIP)
	require.Empty(t, th.ListEndpoints())

	// Labels changed.
	changedLabels := serviceWithClusterIP.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.OnUpdate(serviceWithClusterIP, changedLabels)

	endpoints := th.ListEndpoints()
	require.ElementsMatch(t,
		[]observer.EndpointID{"test-1/service-1-UID"},
		[]observer.EndpointID{endpoints[0].ID},
	)

	// Running state changed, one added and one removed.
	updatedService := serviceWithClusterIP.DeepCopy()
	updatedService.Labels["updated-label"] = "true"
	th.OnUpdate(serviceWithClusterIP, updatedService)
	require.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/service-1-UID",
			Target: "service-1.default.svc.cluster.local",
			Details: &observer.K8sService{
				Name:        "service-1",
				Namespace:   "default",
				UID:         "service-1-UID",
				Labels:      map[string]string{"env": "prod", "updated-label": "true"},
				ServiceType: "ClusterIP",
				ClusterIP:   "1.2.3.4",
			},
		},
	}, th.ListEndpoints())
}

func TestIngressEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(ingress, true)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/ingress-1-UID/host-1/",
			Target: "https://host-1/",
			Details: &observer.K8sIngress{
				Name:      "application-ingress",
				Namespace: "default",
				UID:       "test-1/ingress-1-UID/host-1/",
				Labels:    map[string]string{"env": "prod"},
				Scheme:    "https",
				Host:      "host-1",
				Path:      "/",
			},
		},
	}, th.ListEndpoints())
}

func TestIngressEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(ingress, true)
	th.OnDelete(ingress)
	assert.Empty(t, th.ListEndpoints())
}

func TestIngressEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.OnUpdate(ingress, ingress)
	require.Empty(t, th.ListEndpoints())

	// Labels changed.
	changedLabels := ingress.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.OnUpdate(ingress, changedLabels)

	endpoints := th.ListEndpoints()
	require.ElementsMatch(t,
		[]observer.EndpointID{"test-1/ingress-1-UID/host-1/"},
		[]observer.EndpointID{endpoints[0].ID},
	)

	// Running state changed, one added and one removed.
	updatedIngress := ingress.DeepCopy()
	updatedIngress.Labels["updated-label"] = "true"
	th.OnUpdate(ingress, updatedIngress)
	require.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/ingress-1-UID/host-1/",
			Target: "https://host-1/",
			Details: &observer.K8sIngress{
				Name:      "application-ingress",
				Namespace: "default",
				UID:       "test-1/ingress-1-UID/host-1/",
				Labels:    map[string]string{"env": "prod", "updated-label": "true"},
				Scheme:    "https",
				Host:      "host-1",
				Path:      "/",
			},
		},
	}, th.ListEndpoints())
}

func TestNodeEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(node1V1, true)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/node1-uid",
			Target: "internalIP",
			Details: &observer.K8sNode{
				UID:                 "uid",
				Annotations:         map[string]string{"annotation-key": "annotation-value"},
				Labels:              map[string]string{"label-key": "label-value"},
				Name:                "node1",
				InternalIP:          "internalIP",
				InternalDNS:         "internalDNS",
				Hostname:            "localhost",
				ExternalIP:          "externalIP",
				ExternalDNS:         "externalDNS",
				KubeletEndpointPort: 1234,
			},
		},
	}, th.ListEndpoints())
}

func TestNodeEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.OnAdd(node1V1, true)
	th.OnDelete(node1V1)
	assert.Empty(t, th.ListEndpoints())
}

func TestNodeEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.OnUpdate(node1V1, node1V1)
	require.Empty(t, th.ListEndpoints())

	// Labels changed.
	changedLabels := node1V1.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.OnUpdate(node1V1, changedLabels)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/node1-uid",
			Target: "internalIP",
			Details: &observer.K8sNode{
				UID:         "uid",
				Annotations: map[string]string{"annotation-key": "annotation-value"},
				Labels: map[string]string{
					"label-key": "label-value",
					"new-label": "value",
				},
				Name:                "node1",
				InternalIP:          "internalIP",
				InternalDNS:         "internalDNS",
				Hostname:            "localhost",
				ExternalIP:          "externalIP",
				ExternalDNS:         "externalDNS",
				KubeletEndpointPort: 1234,
			},
		},
	}, th.ListEndpoints())

	// Running state changed, one added and one removed.
	updatedNode := node1V1.DeepCopy()
	updatedNode.Labels["updated-label"] = "true"
	th.OnUpdate(node1V1, updatedNode)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/node1-uid",
			Target: "internalIP",
			Details: &observer.K8sNode{
				UID:         "uid",
				Annotations: map[string]string{"annotation-key": "annotation-value"},
				Labels: map[string]string{
					"label-key":     "label-value",
					"updated-label": "true",
				},
				Name:                "node1",
				InternalIP:          "internalIP",
				InternalDNS:         "internalDNS",
				Hostname:            "localhost",
				ExternalIP:          "externalIP",
				ExternalDNS:         "externalDNS",
				KubeletEndpointPort: 1234,
			},
		},
	}, th.ListEndpoints())
}
