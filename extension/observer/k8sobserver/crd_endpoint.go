// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// unstructuredToEndpoint converts an unstructured Kubernetes object (CRD instance)
// into an observer endpoint. It returns an error when required fields are missing
// or the object cannot be represented as an endpoint.
func unstructuredToEndpoint(idNamespace string, obj *unstructured.Unstructured) (observer.Endpoint, error) {
	if obj == nil {
		return observer.Endpoint{}, errors.New("object is nil")
	}
	if obj.Object == nil {
		return observer.Endpoint{}, errors.New("object has no root fields")
	}
	uid := string(obj.GetUID())
	if uid == "" {
		return observer.Endpoint{}, errors.New("metadata.uid is required for endpoint identity")
	}
	name := obj.GetName()
	if name == "" {
		return observer.Endpoint{}, errors.New("metadata.name is required")
	}

	endpointID := observer.EndpointID(fmt.Sprintf("%s/%s", idNamespace, uid))

	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	status, _, _ := unstructured.NestedMap(obj.Object, "status")

	var specMap map[string]any
	if spec != nil {
		specMap = spec
	}

	var statusMap map[string]any
	if status != nil {
		statusMap = status
	}

	gvk := obj.GroupVersionKind()

	return observer.Endpoint{
		ID:     endpointID,
		Target: name,
		Details: &observer.K8sCRD{
			Name:        name,
			UID:         uid,
			Labels:      obj.GetLabels(),
			Annotations: obj.GetAnnotations(),
			Namespace:   obj.GetNamespace(),
			Group:       gvk.Group,
			Version:     gvk.Version,
			Kind:        gvk.Kind,
			Spec:        specMap,
			Status:      statusMap,
		},
	}, nil
}
