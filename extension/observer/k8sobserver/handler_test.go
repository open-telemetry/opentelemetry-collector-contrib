// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sobserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type testHandler struct {
	h    *handler
	sink *endpointSink
}

func newTestHandler() testHandler {
	sink := &endpointSink{}
	return testHandler{
		h: &handler{
			idNamespace: "test-1",
			listener:    sink,
			logger:      zap.NewNop(),
		},
		sink: sink,
	}
}

func TestPodEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.h.OnAdd(podWithNamedPorts)
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
		}, {
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
		}}, th.sink.added)
	assert.Nil(t, th.sink.removed)
	assert.Nil(t, th.sink.changed)
}

func TestPodEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.h.OnDelete(podWithNamedPorts)
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
		}, {
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https",
				Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod"},
				},
				Port:      443,
				Transport: observer.ProtocolTCP,
			},
		}}, th.sink.removed)
	assert.Nil(t, th.sink.added)
	assert.Nil(t, th.sink.changed)
}

func TestPodEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.h.OnUpdate(podWithNamedPorts, podWithNamedPorts)
	assert.Nil(t, th.sink.added)
	assert.Nil(t, th.sink.changed)
	assert.Nil(t, th.sink.removed)

	// Labels changed.
	changedLabels := podWithNamedPorts.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.h.OnUpdate(podWithNamedPorts, changedLabels)

	assert.Nil(t, th.sink.added)
	assert.Nil(t, th.sink.removed)
	assert.ElementsMatch(t,
		[]observer.EndpointID{"test-1/pod-2-UID", "test-1/pod-2-UID/https(443)"},
		[]observer.EndpointID{th.sink.changed[0].ID, th.sink.changed[1].ID})

	// Running state changed, one added and one removed.
	updatedPod := podWithNamedPorts.DeepCopy()
	updatedPod.Labels["updated-label"] = "true"
	th.h.OnUpdate(podWithNamedPorts, updatedPod)
	assert.Nil(t, th.sink.added)
	assert.Nil(t, th.sink.removed)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/pod-2-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod", "updated-label": "true"}}},
		{
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https", Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod", "updated-label": "true"}},
				Port:      443,
				Transport: observer.ProtocolTCP}},
	}, th.sink.changed)
}

func TestNodeEndpointsAdded(t *testing.T) {
	th := newTestHandler()
	th.h.OnAdd(node1V1)
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
	}, th.sink.added)
	assert.Nil(t, th.sink.removed)
	assert.Nil(t, th.sink.changed)
}

func TestNodeEndpointsRemoved(t *testing.T) {
	th := newTestHandler()
	th.h.OnDelete(node1V1)
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
	}, th.sink.removed)
	assert.Nil(t, th.sink.added)
	assert.Nil(t, th.sink.changed)
}

func TestNodeEndpointsChanged(t *testing.T) {
	th := newTestHandler()
	// Nothing changed.
	th.h.OnUpdate(node1V1, node1V1)
	assert.Nil(t, th.sink.added)
	assert.Nil(t, th.sink.changed)
	assert.Nil(t, th.sink.removed)

	// Labels changed.
	changedLabels := node1V1.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	th.h.OnUpdate(node1V1, changedLabels)

	assert.Nil(t, th.sink.added)
	assert.Nil(t, th.sink.removed)
	assert.ElementsMatch(t,
		[]observer.EndpointID{"test-1/node1-uid"},
		[]observer.EndpointID{th.sink.changed[0].ID})

	// Running state changed, one added and one removed.
	updatedNode := node1V1.DeepCopy()
	updatedNode.Labels["updated-label"] = "true"
	th.h.OnUpdate(podWithNamedPorts, updatedNode)
	assert.Nil(t, th.sink.added)
	assert.Nil(t, th.sink.removed)
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
	}, th.sink.changed)
}
