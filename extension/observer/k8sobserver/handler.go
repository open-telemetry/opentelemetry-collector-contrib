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

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"encoding/json"
	"reflect"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// handler handles k8s cache informer callbacks.
type handler struct {
	// idNamespace should be some unique token to distinguish multiple handler instances.
	idNamespace string
	// listener is the callback for discovered endpoints.
	listener observer.Notify
	logger   *zap.Logger
}

// OnAdd is called in response to a new pod or node being detected.
func (h *handler) OnAdd(objectInterface interface{}) {
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
		if env, err := endpoint.Env(); err == nil {
			if marshaled, err := json.Marshal(env); err == nil {
				h.logger.Debug("endpoint added", zap.String("env", string(marshaled)))
			}
		}
	}

	h.listener.OnAdd(endpoints)
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
			if env, err := endpoint.Env(); err == nil {
				if marshaled, err := json.Marshal(env); err == nil {
					h.logger.Debug("endpoint removed (via update)", zap.String("env", string(marshaled)))
				}
			}
		}
		h.listener.OnRemove(removedEndpoints)
	}

	if len(updatedEndpoints) > 0 {
		for _, endpoint := range updatedEndpoints {
			if env, err := endpoint.Env(); err == nil {
				if marshaled, err := json.Marshal(env); err == nil {
					h.logger.Debug("endpoint changed (via update)", zap.String("env", string(marshaled)))
				}
			}
		}
		h.listener.OnChange(updatedEndpoints)
	}

	if len(addedEndpoints) > 0 {
		for _, endpoint := range addedEndpoints {
			if env, err := endpoint.Env(); err == nil {
				if marshaled, err := json.Marshal(env); err == nil {
					h.logger.Debug("endpoint added (via update)", zap.String("env", string(marshaled)))
				}
			}
		}
		h.listener.OnAdd(addedEndpoints)
	}

	// TODO: can changes be missed where a pod is deleted but we don't
	// send remove notifications for some of its endpoints? If not provable
	// then maybe keep track of pod -> endpoint association to be sure
	// they are all cleaned up.
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
			if env, err := endpoint.Env(); err == nil {
				if marshaled, err := json.Marshal(env); err == nil {
					h.logger.Debug("endpoint deleted", zap.String("env", string(marshaled)))
				}
			}
		}
		h.listener.OnRemove(endpoints)
	}
}
