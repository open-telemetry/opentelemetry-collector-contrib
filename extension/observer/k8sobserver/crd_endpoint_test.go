// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestConvertUnstructuredToEndpoint(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "mycompany.io/v1",
			"kind":       "MyResource",
			"metadata": map[string]any{
				"name":      "my-resource-1",
				"namespace": "test-namespace",
				"uid":       "test-uid-12345",
				"labels": map[string]any{
					"app":     "myapp",
					"version": "v1",
				},
				"annotations": map[string]any{
					"description": "A test resource",
				},
			},
			"spec": map[string]any{
				"replicas": int64(3),
				"image":    "myimage:latest",
				"config": map[string]any{
					"port": int64(8080),
				},
			},
			"status": map[string]any{
				"phase":    "Running",
				"replicas": int64(3),
			},
		},
	}
	obj.SetGroupVersionKind(obj.GroupVersionKind())
	obj.SetUID(types.UID("test-uid-12345"))

	endpoint := convertUnstructuredToEndpoint("test-namespace", obj)

	assert.Equal(t, observer.EndpointID("test-namespace/test-uid-12345"), endpoint.ID)
	assert.Equal(t, "my-resource-1", endpoint.Target)

	require.NotNil(t, endpoint.Details)
	crdDetails, ok := endpoint.Details.(*observer.K8sCRD)
	require.True(t, ok, "Expected endpoint details to be *observer.K8sCRD")

	assert.Equal(t, "my-resource-1", crdDetails.Name)
	assert.Equal(t, "test-uid-12345", crdDetails.UID)
	assert.Equal(t, "test-namespace", crdDetails.Namespace)
	assert.Equal(t, "mycompany.io", crdDetails.Group)
	assert.Equal(t, "v1", crdDetails.Version)
	assert.Equal(t, "MyResource", crdDetails.Kind)

	assert.Equal(t, map[string]string{
		"app":     "myapp",
		"version": "v1",
	}, crdDetails.Labels)

	assert.Equal(t, map[string]string{
		"description": "A test resource",
	}, crdDetails.Annotations)

	// Check spec
	require.NotNil(t, crdDetails.Spec)
	assert.Equal(t, int64(3), crdDetails.Spec["replicas"])
	assert.Equal(t, "myimage:latest", crdDetails.Spec["image"])

	// Check status
	require.NotNil(t, crdDetails.Status)
	assert.Equal(t, "Running", crdDetails.Status["phase"])
	assert.Equal(t, int64(3), crdDetails.Status["replicas"])

	// Check Env() returns correct values
	env := crdDetails.Env()
	assert.Equal(t, "my-resource-1", env["name"])
	assert.Equal(t, "test-uid-12345", env["uid"])
	assert.Equal(t, "test-namespace", env["namespace"])
	assert.Equal(t, "mycompany.io", env["group"])
	assert.Equal(t, "v1", env["version"])
	assert.Equal(t, "MyResource", env["kind"])

	// Check Type() returns correct type
	assert.Equal(t, observer.K8sCRDType, crdDetails.Type())
}

func TestConvertUnstructuredToEndpointClusterScoped(t *testing.T) {
	t.Parallel()

	// Test a cluster-scoped resource (no namespace)
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "cluster.example.com/v1",
			"kind":       "ClusterResource",
			"metadata": map[string]any{
				"name": "cluster-resource-1",
				"uid":  "cluster-uid-67890",
			},
			"spec": map[string]any{
				"global": true,
			},
		},
	}
	obj.SetGroupVersionKind(obj.GroupVersionKind())
	obj.SetUID(types.UID("cluster-uid-67890"))

	endpoint := convertUnstructuredToEndpoint("test-observer", obj)

	assert.Equal(t, observer.EndpointID("test-observer/cluster-uid-67890"), endpoint.ID)
	assert.Equal(t, "cluster-resource-1", endpoint.Target)

	crdDetails, ok := endpoint.Details.(*observer.K8sCRD)
	require.True(t, ok)

	assert.Equal(t, "", crdDetails.Namespace) // Cluster-scoped, no namespace
	assert.Equal(t, "cluster.example.com", crdDetails.Group)
	assert.Equal(t, "v1", crdDetails.Version)
	assert.Equal(t, "ClusterResource", crdDetails.Kind)
}

func TestConvertUnstructuredToEndpointMinimal(t *testing.T) {
	t.Parallel()

	// Test minimal CRD with no spec or status
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "minimal.io/v1beta1",
			"kind":       "Minimal",
			"metadata": map[string]any{
				"name":      "minimal-1",
				"namespace": "default",
				"uid":       "minimal-uid",
			},
		},
	}
	obj.SetGroupVersionKind(obj.GroupVersionKind())
	obj.SetUID(types.UID("minimal-uid"))

	endpoint := convertUnstructuredToEndpoint("obs", obj)

	crdDetails, ok := endpoint.Details.(*observer.K8sCRD)
	require.True(t, ok)

	assert.Nil(t, crdDetails.Spec)
	assert.Nil(t, crdDetails.Status)
	assert.Nil(t, crdDetails.Labels)
	assert.Nil(t, crdDetails.Annotations)
}
