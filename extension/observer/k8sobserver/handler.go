// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"reflect"
	"sync"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
)

func crdLogFields(obj *unstructured.Unstructured) []zap.Field {
	if obj == nil {
		return nil
	}
	return []zap.Field{
		zap.String("api_version", obj.GetAPIVersion()),
		zap.String("kind", obj.GetKind()),
		zap.String("name", obj.GetName()),
		zap.String("namespace", obj.GetNamespace()),
		zap.String("uid", string(obj.GetUID())),
	}
}

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

	logger *zap.Logger
}

func (h *handler) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint
	h.endpoints.Range(func(endpointID, endpoint any) bool {
		if e, ok := endpoint.(observer.Endpoint); ok {
			endpoints = append(endpoints, e)
		} else {
			h.logger.Info("failed listing endpoint", zap.Any("endpointID", endpointID), zap.Any("endpoint", endpoint))
		}
		return true
	})
	return endpoints
}

// OnAdd is called by SharedInformer when a watched object is added or re-synced from a list.
func (h *handler) OnAdd(objectInterface any, _ bool) {
	var endpoints []observer.Endpoint

	switch object := objectInterface.(type) {
	case *v1.Pod:
		endpoints = convertPodToEndpoints(h.idNamespace, object)
	case *v1.Service:
		endpoints = convertServiceToEndpoints(h.idNamespace, object)
	case *networkingv1.Ingress:
		endpoints = convertIngressToEndpoints(h.idNamespace, object)
	case *v1.Node:
		endpoints = append(endpoints, convertNodeToEndpoint(h.idNamespace, object))
	case *unstructured.Unstructured:
		if object == nil {
			h.logger.Warn("skipping custom resource: nil unstructured object on add")
			return
		}
		ep, err := unstructuredToEndpoint(h.idNamespace, object)
		if err != nil {
			fields := append([]zap.Field{zap.Error(err)}, crdLogFields(object)...)
			h.logger.Warn("skipping custom resource: cannot build observer endpoint", fields...)
			return
		}
		endpoints = append(endpoints, ep)
	default: // unsupported
		return
	}

	for _, endpoint := range endpoints {
		h.endpoints.Store(endpoint.ID, endpoint)
	}
}

// OnUpdate is called by SharedInformer when a watched object changes.
func (h *handler) OnUpdate(oldObjectInterface, newObjectInterface any) {
	switch oldObject := oldObjectInterface.(type) {
	case *unstructured.Unstructured:
		h.applyUnstructuredUpdate(oldObject, newObjectInterface)
		return
	case *v1.Pod:
		newPod, ok := newObjectInterface.(*v1.Pod)
		if !ok {
			h.logger.Warn("skip updating endpoint for pod as the update is of different type", zap.Any("oldPod", oldObjectInterface), zap.Any("newObject", newObjectInterface))
			return
		}
		h.applyEndpointDiff(
			convertPodToEndpoints(h.idNamespace, oldObject),
			convertPodToEndpoints(h.idNamespace, newPod),
		)
		return
	case *v1.Service:
		newService, ok := newObjectInterface.(*v1.Service)
		if !ok {
			h.logger.Warn("skip updating endpoint for service as the update is of different type", zap.Any("oldService", oldObjectInterface), zap.Any("newObject", newObjectInterface))
			return
		}
		h.applyEndpointDiff(
			convertServiceToEndpoints(h.idNamespace, oldObject),
			convertServiceToEndpoints(h.idNamespace, newService),
		)
		return
	case *networkingv1.Ingress:
		newIngress, ok := newObjectInterface.(*networkingv1.Ingress)
		if !ok {
			h.logger.Warn("skip updating endpoint for ingress as the update is of different type", zap.Any("oldIngress", oldObjectInterface), zap.Any("newObject", newObjectInterface))
			return
		}
		h.applyEndpointDiff(
			convertIngressToEndpoints(h.idNamespace, oldObject),
			convertIngressToEndpoints(h.idNamespace, newIngress),
		)
		return
	case *v1.Node:
		newNode, ok := newObjectInterface.(*v1.Node)
		if !ok {
			h.logger.Warn("skip updating endpoint for node as the update is of different type", zap.Any("oldNode", oldObjectInterface), zap.Any("newObject", newObjectInterface))
			return
		}
		oldEndpoint := convertNodeToEndpoint(h.idNamespace, oldObject)
		newEndpoint := convertNodeToEndpoint(h.idNamespace, newNode)
		h.applyEndpointDiff([]observer.Endpoint{oldEndpoint}, []observer.Endpoint{newEndpoint})
		return
	default: // unsupported
		return
	}
}

func (h *handler) applyUnstructuredUpdate(oldObj *unstructured.Unstructured, newObjInterface any) {
	if oldObj == nil {
		h.logger.Warn("skipping custom resource update: nil old unstructured object")
		return
	}
	newCRD, ok := newObjInterface.(*unstructured.Unstructured)
	if !ok {
		h.logger.Warn("skip updating endpoint for CRD as the update is of different type", zap.Any("oldCRD", oldObj), zap.Any("newObject", newObjInterface))
		return
	}
	if newCRD == nil {
		h.logger.Warn("skipping custom resource update: nil new unstructured object")
		return
	}

	oldEndpoint, oldErr := unstructuredToEndpoint(h.idNamespace, oldObj)
	newEndpoint, newErr := unstructuredToEndpoint(h.idNamespace, newCRD)
	if oldErr != nil {
		fields := append([]zap.Field{zap.Error(oldErr)}, crdLogFields(oldObj)...)
		h.logger.Warn("skipping custom resource update: old object invalid", fields...)
	}
	if newErr != nil {
		fields := append([]zap.Field{zap.Error(newErr)}, crdLogFields(newCRD)...)
		h.logger.Warn("skipping custom resource update: new object invalid", fields...)
	}
	if oldErr != nil && newErr != nil {
		return
	}
	if oldErr != nil && newErr == nil {
		h.endpoints.Store(newEndpoint.ID, newEndpoint)
		return
	}
	if oldErr == nil && newErr != nil {
		h.endpoints.Delete(oldEndpoint.ID)
		return
	}
	if reflect.DeepEqual(oldEndpoint, newEndpoint) {
		return
	}
	h.endpoints.Store(newEndpoint.ID, newEndpoint)
}

func (h *handler) applyEndpointDiff(oldEndpointsList, newEndpointsList []observer.Endpoint) {
	oldEndpoints := map[observer.EndpointID]observer.Endpoint{}
	newEndpoints := map[observer.EndpointID]observer.Endpoint{}

	for _, e := range oldEndpointsList {
		oldEndpoints[e.ID] = e
	}
	for _, e := range newEndpointsList {
		newEndpoints[e.ID] = e
	}

	var removedEndpoints, updatedEndpoints, addedEndpoints []observer.Endpoint

	for _, e := range newEndpoints {
		if existing, ok := oldEndpoints[e.ID]; ok {
			if !reflect.DeepEqual(existing, e) {
				updatedEndpoints = append(updatedEndpoints, e)
			}
		} else {
			addedEndpoints = append(addedEndpoints, e)
		}
	}

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

// OnDelete is called by SharedInformer when a watched object is deleted.
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
			endpoints = convertPodToEndpoints(h.idNamespace, object)
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
	case *unstructured.Unstructured:
		if object == nil {
			h.logger.Warn("skipping custom resource delete: nil unstructured object")
			return
		}
		endpoint, err := unstructuredToEndpoint(h.idNamespace, object)
		if err != nil {
			fields := append([]zap.Field{zap.Error(err)}, crdLogFields(object)...)
			h.logger.Warn("skipping custom resource delete: cannot resolve endpoint id", fields...)
			return
		}
		endpoints = append(endpoints, endpoint)
	default: // unsupported
		return
	}
	if len(endpoints) != 0 {
		for _, endpoint := range endpoints {
			h.endpoints.Delete(endpoint.ID)
		}
	}
}
