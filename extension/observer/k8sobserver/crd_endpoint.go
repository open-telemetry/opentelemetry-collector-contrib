// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// convertUnstructuredToEndpoint converts an unstructured Kubernetes object (CRD instance)
// into an observer endpoint.
func convertUnstructuredToEndpoint(idNamespace string, obj *unstructured.Unstructured) observer.Endpoint {
	uid := string(obj.GetUID())
	endpointID := observer.EndpointID(fmt.Sprintf("%s/%s", idNamespace, uid))

	// Extract spec and status as maps
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	status, _, _ := unstructured.NestedMap(obj.Object, "status")

	// Convert spec and status to map[string]any
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
		Target: obj.GetName(),
		Details: &observer.K8sCRD{
			Name:        obj.GetName(),
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
	}
}
