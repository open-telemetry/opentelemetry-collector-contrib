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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestEndpointsAdded(t *testing.T) {
	sink := endpointSink{}
	h := handler{
		idNamespace: "test-1",
		watcher:     &sink,
	}
	h.OnAdd(podWithNamedPorts)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/pod-2-UID",
			Target: "1.2.3.4",
			Details: observer.Pod{
				Name:   "pod-2",
				Labels: map[string]string{"env": "prod"},
			},
		}, {
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: observer.Port{
				Name: "https",
				Pod: observer.Pod{
					Name:   "pod-2",
					Labels: map[string]string{"env": "prod"},
				},
				Port:     443,
				Protocol: observer.ProtocolTCP,
			},
		}}, sink.added)
	assert.Nil(t, sink.removed)
	assert.Nil(t, sink.changed)
}

func TestEndpointsRemoved(t *testing.T) {
	sink := endpointSink{}
	h := handler{
		idNamespace: "test-1",
		watcher:     &sink,
	}
	h.OnDelete(podWithNamedPorts)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/pod-2-UID",
			Target: "1.2.3.4",
			Details: observer.Pod{
				Name:   "pod-2",
				Labels: map[string]string{"env": "prod"},
			},
		}, {
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: observer.Port{
				Name: "https",
				Pod: observer.Pod{
					Name:   "pod-2",
					Labels: map[string]string{"env": "prod"},
				},
				Port:     443,
				Protocol: observer.ProtocolTCP,
			},
		}}, sink.removed)
	assert.Nil(t, sink.added)
	assert.Nil(t, sink.changed)
}

func TestEndpointsChanged(t *testing.T) {
	sink := endpointSink{}
	h := handler{
		idNamespace: "test-1",
		watcher:     &sink,
	}
	// Nothing changed.
	h.OnUpdate(podWithNamedPorts, podWithNamedPorts)
	assert.Nil(t, sink.added)
	assert.Nil(t, sink.changed)
	assert.Nil(t, sink.removed)

	// Labels changed.
	changedLabels := podWithNamedPorts.DeepCopy()
	changedLabels.Labels["new-label"] = "value"
	h.OnUpdate(podWithNamedPorts, changedLabels)

	assert.Nil(t, sink.added)
	assert.Nil(t, sink.removed)
	assert.ElementsMatch(t,
		[]string{"test-1/pod-2-UID", "test-1/pod-2-UID/https(443)"},
		[]string{sink.changed[0].ID, sink.changed[1].ID})

	// Running state changed, one added and one removed.
	sink = endpointSink{}
	updatedPod := podWithNamedPorts.DeepCopy()
	updatedPod.Labels["updated-label"] = "true"
	h.OnUpdate(podWithNamedPorts, updatedPod)
	assert.Nil(t, sink.added)
	assert.Nil(t, sink.removed)
	assert.ElementsMatch(t, []observer.Endpoint{
		{
			ID:     "test-1/pod-2-UID",
			Target: "1.2.3.4",
			Details: observer.Pod{
				Name:   "pod-2",
				Labels: map[string]string{"env": "prod", "updated-label": "true"}}},
		{
			ID:     "test-1/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: observer.Port{
				Name: "https", Pod: observer.Pod{
					Name:   "pod-2",
					Labels: map[string]string{"env": "prod", "updated-label": "true"}},
				Port:     443,
				Protocol: observer.ProtocolTCP}},
	}, sink.changed)
}
