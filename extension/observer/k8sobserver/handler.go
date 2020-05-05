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
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// handler handles k8s cache informer callbacks.
type handler struct {
	// idNamespace should be some unique token to distinguish multiple handler instances.
	idNamespace string
	// watcher is the callback for discovered endpoints.
	watcher observer.Notify
}

// OnAdd is called in response to a pod being added.
func (h *handler) OnAdd(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}
	h.watcher.OnAdd(h.convertPodToEndpoints(pod))
}

// convertPodToEndpoints converts a pod instance into a slice of endpoints. The endpoints
// include the pod itself as well as an endpoint for each container port that is mapped
// to a container that is in a running state.
func (h *handler) convertPodToEndpoints(pod *v1.Pod) []observer.Endpoint {
	podID := fmt.Sprintf("%s/%s", h.idNamespace, pod.UID)
	podIP := pod.Status.PodIP
	labels := map[string]string{}

	for k, v := range pod.Labels {
		labels[k] = v
	}
	for k, v := range pod.Annotations {
		labels[k] = v
	}

	podDetails := observer.Pod{
		Labels: labels,
		Name:   pod.Name,
	}

	endpoints := []observer.Endpoint{{
		ID:      podID,
		Target:  podIP,
		Details: podDetails,
	}}

	// Map of running containers by name.
	containerRunning := map[string]bool{}

	for _, container := range pod.Status.ContainerStatuses {
		if container.State.Running != nil {
			containerRunning[container.Name] = true
		}
	}

	// Create endpoint for each named container port.
	for _, container := range pod.Spec.Containers {
		if !containerRunning[container.Name] {
			continue
		}

		for _, port := range container.Ports {
			endpoints = append(endpoints, observer.Endpoint{
				ID:     fmt.Sprintf("%s/%s(%d)", podID, port.Name, port.ContainerPort),
				Target: fmt.Sprintf("%s:%d", podIP, port.ContainerPort),
				Details: observer.Port{
					Pod:      podDetails,
					Name:     port.Name,
					Port:     uint16(port.ContainerPort),
					Protocol: getProtocol(port.Protocol),
				},
			})
		}
	}

	return endpoints
}

func getProtocol(protocol v1.Protocol) observer.Protocol {
	switch protocol {
	case v1.ProtocolTCP:
		return observer.ProtocolTCP
	case v1.ProtocolUDP:
		return observer.ProtocolUDP
	}
	return observer.ProtocolUnknown
}

// OnUpdate is called in response to an existing pod changing.
func (h *handler) OnUpdate(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		return
	}
	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		return
	}

	oldEndpoints := map[string]observer.Endpoint{}
	newEndpoints := map[string]observer.Endpoint{}

	// Convert pods to endpoints and map by ID for easier lookup.
	for _, e := range h.convertPodToEndpoints(oldPod) {
		oldEndpoints[e.ID] = e
	}
	for _, e := range h.convertPodToEndpoints(newPod) {
		newEndpoints[e.ID] = e
	}

	var removedEndpoints, updatedEndpoints, addedEndpoints []observer.Endpoint

	// Find endpoints that are present in oldPod and newPod and see if they've
	// changed. Otherwise if it wasn't in oldPod it's a new endpoint.
	for _, e := range newEndpoints {
		if existing, ok := oldEndpoints[e.ID]; ok {
			if !reflect.DeepEqual(existing, e) {
				updatedEndpoints = append(updatedEndpoints, e)
			}
		} else {
			addedEndpoints = append(addedEndpoints, e)
		}
	}

	// If an endpoint is present in the oldPod but not in the newPod then
	// send as removed.
	for _, e := range oldEndpoints {
		if _, ok := newEndpoints[e.ID]; !ok {
			removedEndpoints = append(removedEndpoints, e)
		}
	}

	if len(removedEndpoints) > 0 {
		h.watcher.OnRemove(removedEndpoints)
	}

	if len(updatedEndpoints) > 0 {
		h.watcher.OnChange(updatedEndpoints)
	}

	if len(addedEndpoints) > 0 {
		h.watcher.OnAdd(addedEndpoints)
	}

	// TODO: can changes be missed where a pod is deleted but we don't
	// send remove notifications for some of its endpoints? If not provable
	// then maybe keep track of pod -> endpoint association to be sure
	// they are all cleaned up.
}

// OnDelete is called in response to a pod being deleted.
func (h *handler) OnDelete(obj interface{}) {
	var pod *v1.Pod
	switch o := obj.(type) {
	case *cache.DeletedFinalStateUnknown:
		// Assuming we never saw the pod state where new endpoints would have been created
		// to begin with it seems that we can't leak endpoints here.
		pod = o.Obj.(*v1.Pod)
	case *v1.Pod:
		pod = o
	}
	if pod == nil {
		return
	}
	h.watcher.OnRemove(h.convertPodToEndpoints(pod))
}
