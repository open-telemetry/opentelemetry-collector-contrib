// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
)

// deletedPodLinger is how long the container endpoints of a deleted pod stay listed when
// terminated containers are observed. The endpoints are read once a second, so without it a pod
// that is gone within that second is never seen, and a receiver gets no time to read what the
// containers left.
const deletedPodLinger = 10 * time.Second

var (
	_ cache.ResourceEventHandler       = (*handler)(nil)
	_ endpointswatcher.EndpointsLister = (*handler)(nil)
)

// handler handles k8s cache informer callbacks.
type handler struct {
	// idNamespace should be some unique token to distinguish multiple handler instances.
	idNamespace string
	// endpoints is a map[observer.EndpointID]observer.Endpoint all existing endpoints at any given moment
	endpoints *sync.Map
	// podOptions selects the containers of a pod that get endpoints.
	podOptions podEndpointOptions
	// terminatedContainerTTL is how long the endpoint of a container that is not running stays
	// listed after the container finished. Zero keeps it until the pod is deleted.
	terminatedContainerTTL time.Duration
	// expiries is a map[observer.EndpointID]expiry of the container endpoints that stop being listed.
	expiries sync.Map
	now      func() time.Time

	logger *zap.Logger
}

// expiry is the time from which an endpoint is not listed.
type expiry struct {
	at time.Time
	// remove tells to drop the endpoint then, as its pod is deleted.
	remove bool
}

func (h *handler) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint
	now := h.now()
	h.endpoints.Range(func(endpointID, endpoint any) bool {
		if e, ok := endpoint.(observer.Endpoint); ok {
			expires, found := h.expiries.Load(endpointID)
			if found && !now.Before(expires.(expiry).at) {
				if expires.(expiry).remove {
					h.endpoints.Delete(endpointID)
					h.expiries.Delete(endpointID)
				}
				return true
			}
			// The time an endpoint expires is stored before the endpoint and removed after it.
			// An endpoint of a container that is not running and that has none can so be one
			// that was removed just now.
			if container, ok := e.Details.(*observer.PodContainer); ok && !found && !container.Running {
				if _, stored := h.endpoints.Load(endpointID); !stored {
					return true
				}
			}
			endpoints = append(endpoints, e)
		} else {
			h.logger.Info("failed listing endpoint", zap.Any("endpointID", endpointID), zap.Any("endpoint", endpoint))
		}
		return true
	})
	return endpoints
}

// OnAdd is called in response to a new pod or node being detected.
func (h *handler) OnAdd(objectInterface any, _ bool) {
	var endpoints []observer.Endpoint

	switch object := objectInterface.(type) {
	case *v1.Pod:
		endpoints = convertPodToEndpoints(h.idNamespace, object, h.podOptions)
	case *v1.Service:
		endpoints = convertServiceToEndpoints(h.idNamespace, object)
	case *networkingv1.Ingress:
		endpoints = convertIngressToEndpoints(h.idNamespace, object)
	case *v1.Node:
		endpoints = append(endpoints, convertNodeToEndpoint(h.idNamespace, object))
	default: // unsupported
		return
	}

	h.removeExpiredEndpoints()
	pod, _ := objectInterface.(*v1.Pod)
	if pod != nil {
		h.expireTerminatedContainers(pod, endpoints, true)
	}
	for _, endpoint := range endpoints {
		h.endpoints.Store(endpoint.ID, endpoint)
	}
	if pod != nil {
		h.expireTerminatedContainers(pod, endpoints, false)
	}
}

// expireTerminatedContainers sets when the endpoints of the containers of a pod that are not
// running stop being listed. It is called with expire set before the endpoints are stored and
// without it after that, so that a listing in between never has an endpoint that is to expire
// without the time it expires.
func (h *handler) expireTerminatedContainers(pod *v1.Pod, endpoints []observer.Endpoint, expire bool) {
	if !h.podOptions.terminatedContainers {
		return
	}
	finished := containerFinishTimes(h.idNamespace, pod)
	for _, endpoint := range endpoints {
		container, ok := endpoint.Details.(*observer.PodContainer)
		if !ok {
			continue
		}
		expires := !container.Running && h.terminatedContainerTTL > 0
		switch {
		case expire && expires:
			if finishedAt, known := finished[endpoint.ID]; known {
				h.expiries.Store(endpoint.ID, expiry{at: finishedAt.Add(h.terminatedContainerTTL)})
			} else {
				// The status does not say when the container finished, so the time it was first
				// seen not running counts.
				h.expiries.LoadOrStore(endpoint.ID, expiry{at: h.now().Add(h.terminatedContainerTTL)})
			}
		case !expire && !expires:
			h.expiries.Delete(endpoint.ID)
		}
	}
}

// notRunning returns the endpoint of a container as that of a container that is not running.
func notRunning(endpoint observer.Endpoint) observer.Endpoint {
	if container, ok := endpoint.Details.(*observer.PodContainer); ok && container.Running {
		stopped := *container
		stopped.Running = false
		endpoint.Details = &stopped
	}
	return endpoint
}

// removeExpiredEndpoints drops the endpoints of deleted pods that stayed listed for
// deletedPodLinger. ListEndpoints drops them too, but nothing calls it while no one watches the
// observer.
func (h *handler) removeExpiredEndpoints() {
	now := h.now()
	h.expiries.Range(func(endpointID, expires any) bool {
		if e := expires.(expiry); e.remove && !now.Before(e.at) {
			h.endpoints.Delete(endpointID)
			h.expiries.Delete(endpointID)
		}
		return true
	})
}

// lingers tells whether an endpoint of a deleted pod stays listed for deletedPodLinger.
func (h *handler) lingers(endpoint observer.Endpoint) bool {
	if !h.podOptions.terminatedContainers || endpoint.Details.Type() != observer.PodContainerType {
		return false
	}
	if _, ok := h.endpoints.Load(endpoint.ID); !ok {
		return false
	}
	expires, ok := h.expiries.Load(endpoint.ID)
	return !ok || h.now().Before(expires.(expiry).at)
}

// OnUpdate is called in response to an existing pod or node changing.
func (h *handler) OnUpdate(oldObjectInterface, newObjectInterface any) {
	oldEndpoints := map[observer.EndpointID]observer.Endpoint{}
	newEndpoints := map[observer.EndpointID]observer.Endpoint{}
	var updatedPod *v1.Pod
	var podEndpoints []observer.Endpoint

	switch oldObject := oldObjectInterface.(type) {
	case *v1.Pod:
		newPod, ok := newObjectInterface.(*v1.Pod)
		if !ok {
			h.logger.Warn("skip updating endpoint for pod as the update is of different type", zap.Any("oldPod", oldObjectInterface), zap.Any("newObject", newObjectInterface))
			return
		}
		for _, e := range convertPodToEndpoints(h.idNamespace, oldObject, h.podOptions) {
			oldEndpoints[e.ID] = e
		}
		updatedPod = newPod
		podEndpoints = convertPodToEndpoints(h.idNamespace, newPod, h.podOptions)
		for _, e := range podEndpoints {
			newEndpoints[e.ID] = e
		}

	case *v1.Service:
		newService, ok := newObjectInterface.(*v1.Service)
		if !ok {
			h.logger.Warn("skip updating endpoint for service as the update is of different type", zap.Any("oldService", oldObjectInterface), zap.Any("newObject", newObjectInterface))
			return
		}
		for _, e := range convertServiceToEndpoints(h.idNamespace, oldObject) {
			oldEndpoints[e.ID] = e
		}
		for _, e := range convertServiceToEndpoints(h.idNamespace, newService) {
			newEndpoints[e.ID] = e
		}

	case *networkingv1.Ingress:
		newIngress, ok := newObjectInterface.(*networkingv1.Ingress)
		if !ok {
			h.logger.Warn("skip updating endpoint for ingress as the update is of different type", zap.Any("oldIngress", oldObjectInterface), zap.Any("newObject", newObjectInterface))
			return
		}
		for _, e := range convertIngressToEndpoints(h.idNamespace, oldObject) {
			oldEndpoints[e.ID] = e
		}
		for _, e := range convertIngressToEndpoints(h.idNamespace, newIngress) {
			newEndpoints[e.ID] = e
		}

	case *v1.Node:
		newNode, ok := newObjectInterface.(*v1.Node)
		if !ok {
			h.logger.Warn("skip updating endpoint for node as the update is of different type", zap.Any("oldNode", oldObjectInterface), zap.Any("newObject", newObjectInterface))
			return
		}
		oldEndpoint := convertNodeToEndpoint(h.idNamespace, oldObject)
		oldEndpoints[oldEndpoint.ID] = oldEndpoint
		newEndpoint := convertNodeToEndpoint(h.idNamespace, newNode)
		newEndpoints[newEndpoint.ID] = newEndpoint
	default: // unsupported
		return
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
		for _, endpoint := range removedEndpoints {
			h.endpoints.Delete(endpoint.ID)
			h.expiries.Delete(endpoint.ID)
		}
	}

	if updatedPod != nil {
		h.expireTerminatedContainers(updatedPod, podEndpoints, true)
	}

	if len(updatedEndpoints) > 0 {
		for _, endpoint := range updatedEndpoints {
			h.endpoints.Store(endpoint.ID, endpoint)
		}
	}

	if len(addedEndpoints) > 0 {
		for _, endpoint := range addedEndpoints {
			h.endpoints.Store(endpoint.ID, endpoint)
		}
	}

	if updatedPod != nil {
		h.expireTerminatedContainers(updatedPod, podEndpoints, false)
	}
}

// OnDelete is called in response to a pod or node being deleted.
func (h *handler) OnDelete(objectInterface any) {
	var endpoints []observer.Endpoint

	switch object := objectInterface.(type) {
	case *cache.DeletedFinalStateUnknown:
		// Assuming we never saw the pod state where new endpoints would have been created
		// to begin with it seems that we can't leak endpoints here.
		h.OnDelete(object.Obj)
		return
	case *v1.Pod:
		if object != nil {
			endpoints = convertPodToEndpoints(h.idNamespace, object, h.podOptions)
		}
	case *v1.Service:
		if object != nil {
			endpoints = convertServiceToEndpoints(h.idNamespace, object)
		}
	case *networkingv1.Ingress:
		if object != nil {
			endpoints = convertIngressToEndpoints(h.idNamespace, object)
		}
	case *v1.Node:
		if object != nil {
			endpoints = append(endpoints, convertNodeToEndpoint(h.idNamespace, object))
		}
	default: // unsupported
		return
	}
	h.removeExpiredEndpoints()
	if len(endpoints) != 0 {
		for _, endpoint := range endpoints {
			if h.lingers(endpoint) {
				h.expiries.Store(endpoint.ID, expiry{at: h.now().Add(deletedPodLinger), remove: true})
				// The containers of a deleted pod do not run.
				h.endpoints.Store(endpoint.ID, notRunning(endpoint))
				continue
			}
			h.endpoints.Delete(endpoint.ID)
			h.expiries.Delete(endpoint.ID)
		}
	}
}
