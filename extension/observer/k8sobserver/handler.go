// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"reflect"
	"sync"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ cache.ResourceEventHandler = (*handler)(nil)
var _ observer.EndpointsLister = (*handler)(nil)

// handler handles k8s cache informer callbacks.
type handler struct {
	// idNamespace should be some unique token to distinguish multiple handler instances.
	idNamespace string
	// endpoints is a map[observer.EndpointID]observer.Endpoint all existing endpoints at any given moment
	endpoints *sync.Map

	logger *zap.Logger
}

func (h *handler) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint
	h.endpoints.Range(func(endpointID, endpoint interface{}) bool {
		if e, ok := endpoint.(observer.Endpoint); ok {
			endpoints = append(endpoints, e)
		} else {
			h.logger.Info("failed listing endpoint", zap.Any("endpointID", endpointID), zap.Any("endpoint", endpoint))
		}
		return true
	})
	return endpoints
}

// OnAdd is called in response to a new pod or node being detected.
func (h *handler) OnAdd(objectInterface interface{}, isInitialList bool) {
	var endpoints []observer.Endpoint

	switch object := objectInterface.(type) {
	case *v1.Pod:
		endpoints = convertPodToEndpoints(h.idNamespace, object)
	case *v1.Node:
		endpoints = append(endpoints, convertNodeToEndpoint(h.idNamespace, object))
	default: // unsupported
		return
	}

	for _, endpoint := range endpoints {
		h.endpoints.Store(endpoint.ID, endpoint)
	}
}

// OnUpdate is called in response to an existing pod or node changing.
func (h *handler) OnUpdate(oldObjectInterface, newObjectInterface interface{}) {
	oldEndpoints := map[observer.EndpointID]observer.Endpoint{}
	newEndpoints := map[observer.EndpointID]observer.Endpoint{}

	switch oldObject := oldObjectInterface.(type) {
	case *v1.Pod:
		newPod, ok := newObjectInterface.(*v1.Pod)
		if !ok {
			return
		}
		for _, e := range convertPodToEndpoints(h.idNamespace, oldObject) {
			oldEndpoints[e.ID] = e
		}
		for _, e := range convertPodToEndpoints(h.idNamespace, newPod) {
			newEndpoints[e.ID] = e
		}

	case *v1.Node:
		newNode, ok := newObjectInterface.(*v1.Node)
		if !ok {
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
		}
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
}

// OnDelete is called in response to a pod or node being deleted.
func (h *handler) OnDelete(objectInterface interface{}) {
	var endpoints []observer.Endpoint

	switch object := objectInterface.(type) {
	case *cache.DeletedFinalStateUnknown:
		// Assuming we never saw the pod state where new endpoints would have been created
		// to begin with it seems that we can't leak endpoints here.
		h.OnDelete(object.Obj)
		return
	case *v1.Pod:
		if object != nil {
			endpoints = convertPodToEndpoints(h.idNamespace, object)
		}
	case *v1.Node:
		if object != nil {
			endpoints = append(endpoints, convertNodeToEndpoint(h.idNamespace, object))
		}
	default: // unsupported
		return
	}
	if len(endpoints) != 0 {
		for _, endpoint := range endpoints {
			h.endpoints.Delete(endpoint.ID)
		}
	}
}
