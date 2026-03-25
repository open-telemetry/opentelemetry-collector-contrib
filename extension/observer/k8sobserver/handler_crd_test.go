// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	logobserver "go.uber.org/zap/zaptest/observer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestHandlerOnAddInvalidCustomResourceWarnsAndDoesNotStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         any
		wantWarnSub string
	}{
		{
			name:        "nil_unstructured_pointer",
			obj:         (*unstructured.Unstructured)(nil),
			wantWarnSub: "nil unstructured object on add",
		},
		{
			name: "missing_uid",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "example.com/v1",
					"kind":       "Widget",
					"metadata": map[string]any{
						"name":      "w1",
						"namespace": "ns",
					},
				},
			},
			wantWarnSub: "cannot build observer endpoint",
		},
		{
			name: "missing_name",
			obj: func() *unstructured.Unstructured {
				u := &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.com/v1",
						"kind":       "Widget",
						"metadata": map[string]any{
							"namespace": "ns",
							"uid":       "abc",
						},
					},
				}
				u.SetUID(types.UID("abc"))
				return u
			}(),
			wantWarnSub: "cannot build observer endpoint",
		},
		{
			name: "nil_object_root",
			obj: &unstructured.Unstructured{
				Object: nil,
			},
			wantWarnSub: "cannot build observer endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core, logs := logobserver.New(zapcore.WarnLevel)
			logger := zap.New(core)
			h := &handler{
				idNamespace: "test-1",
				endpoints:   &sync.Map{},
				logger:      logger,
			}

			assert.NotPanics(t, func() {
				h.OnAdd(tt.obj, false)
			})

			warnEntries := logs.FilterLevelExact(zapcore.WarnLevel).All()
			require.NotEmpty(t, warnEntries, "expected at least one warning log")
			assert.Contains(t, warnEntries[0].Message, tt.wantWarnSub)

			assert.Empty(t, h.ListEndpoints(), "invalid object must not create endpoints")
		})
	}
}

func TestHandlerOnAddValidCustomResourceRegistersEndpoint(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "acme.example.com/v1",
			"kind":       "Frobber",
			"metadata": map[string]any{
				"name":      "my-frobber",
				"namespace": "production",
				"uid":       "cafe-babe-42",
				"labels": map[string]any{
					"app": "billing",
				},
			},
			"spec": map[string]any{
				"replicas": int64(3),
			},
			"status": map[string]any{
				"ready": true,
			},
		},
	}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "acme.example.com",
		Version: "v1",
		Kind:    "Frobber",
	})
	obj.SetUID(types.UID("cafe-babe-42"))

	h.OnAdd(obj, false)

	endpoints := h.ListEndpoints()
	require.Len(t, endpoints, 1, "OnAdd for a valid CR must register exactly one endpoint")

	ep := endpoints[0]
	assert.Equal(t, observer.EndpointID("test-1/cafe-babe-42"), ep.ID)
	assert.Equal(t, "my-frobber", ep.Target)

	details, ok := ep.Details.(*observer.K8sCRD)
	require.True(t, ok, "details must be *observer.K8sCRD")
	assert.Equal(t, "my-frobber", details.Name)
	assert.Equal(t, "cafe-babe-42", details.UID)
	assert.Equal(t, "production", details.Namespace)
	assert.Equal(t, "acme.example.com", details.Group)
	assert.Equal(t, "v1", details.Version)
	assert.Equal(t, "Frobber", details.Kind)
	assert.Equal(t, map[string]string{"app": "billing"}, details.Labels)
	require.NotNil(t, details.Spec)
	assert.Equal(t, int64(3), details.Spec["replicas"])
	require.NotNil(t, details.Status)
	assert.Equal(t, true, details.Status["ready"])
	assert.Equal(t, observer.K8sCRDType, details.Type())
}

func TestHandlerOnAddClusterScopedCustomResourceRegistersEndpoint(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "cluster.example.com/v1",
			"kind":       "ClusterPolicy",
			"metadata": map[string]any{
				"name": "policy-a",
				"uid":  "cluster-uid-policy",
			},
			"spec": map[string]any{
				"enforce": true,
			},
		},
	}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cluster.example.com",
		Version: "v1",
		Kind:    "ClusterPolicy",
	})
	obj.SetUID(types.UID("cluster-uid-policy"))

	h.OnAdd(obj, false)

	endpoints := h.ListEndpoints()
	require.Len(t, endpoints, 1, "cluster-scoped CR must register one endpoint")

	ep := endpoints[0]
	assert.Equal(t, observer.EndpointID("test-1/cluster-uid-policy"), ep.ID)
	assert.Equal(t, "policy-a", ep.Target)

	details, ok := ep.Details.(*observer.K8sCRD)
	require.True(t, ok)
	assert.Empty(t, details.Namespace, "cluster-scoped instances have no metadata.namespace")
	assert.Equal(t, "cluster.example.com", details.Group)
	assert.Equal(t, "v1", details.Version)
	assert.Equal(t, "ClusterPolicy", details.Kind)
	require.NotNil(t, details.Spec)
	assert.Equal(t, true, details.Spec["enforce"])
}

func TestHandlerOnUpdateValidCustomResourceUpdatesStoredEndpoint(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	oldObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "acme.example.com/v1",
			"kind":       "Frobber",
			"metadata": map[string]any{
				"name":      "x",
				"namespace": "ns",
				"uid":       "uid-99",
			},
			"spec": map[string]any{"version": "1"},
		},
	}
	oldObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "acme.example.com", Version: "v1", Kind: "Frobber"})
	oldObj.SetUID(types.UID("uid-99"))

	newObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "acme.example.com/v1",
			"kind":       "Frobber",
			"metadata": map[string]any{
				"name":      "x",
				"namespace": "ns",
				"uid":       "uid-99",
			},
			"spec": map[string]any{"version": "2"},
		},
	}
	newObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "acme.example.com", Version: "v1", Kind: "Frobber"})
	newObj.SetUID(types.UID("uid-99"))

	h.OnAdd(oldObj, false)
	require.Len(t, h.ListEndpoints(), 1)

	h.OnUpdate(oldObj, newObj)
	endpoints := h.ListEndpoints()
	require.Len(t, endpoints, 1)
	details := endpoints[0].Details.(*observer.K8sCRD)
	assert.Equal(t, "2", details.Spec["version"])
}

func TestHandlerOnDeleteValidCustomResourceRemovesEndpoint(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "acme.example.com/v1",
			"kind":       "Frobber",
			"metadata": map[string]any{
				"name": "gone",
				"uid":  "uid-gone",
			},
		},
	}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "acme.example.com", Version: "v1", Kind: "Frobber"})
	obj.SetUID(types.UID("uid-gone"))

	h.OnAdd(obj, false)
	require.Len(t, h.ListEndpoints(), 1)

	h.OnDelete(obj)
	assert.Empty(t, h.ListEndpoints())
}

func TestHandlerOnDeleteInvalidCustomResourceWarnsAndDoesNotPanic(t *testing.T) {
	t.Parallel()

	core, logs := logobserver.New(zapcore.WarnLevel)
	logger := zap.New(core)
	h := &handler{
		idNamespace: "test-1",
		endpoints:   &sync.Map{},
		logger:      logger,
	}

	bad := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "w1",
				"namespace": "ns",
			},
		},
	}

	assert.NotPanics(t, func() {
		h.OnDelete(bad)
	})

	warnEntries := logs.FilterLevelExact(zapcore.WarnLevel).All()
	require.NotEmpty(t, warnEntries)
	assert.Contains(t, warnEntries[0].Message, "cannot resolve endpoint id")
}

func TestHandlerOnUpdateInvalidNewObjectWarnsRemovesOldAndDoesNotPanic(t *testing.T) {
	t.Parallel()

	core, logs := logobserver.New(zapcore.WarnLevel)
	logger := zap.New(core)
	h := &handler{
		idNamespace: "test-1",
		endpoints:   &sync.Map{},
		logger:      logger,
	}

	oldObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "w1",
				"namespace": "ns",
				"uid":       "uid-1",
			},
		},
	}
	oldObj.SetUID(types.UID("uid-1"))

	newObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "w1",
				"namespace": "ns",
			},
		},
	}

	h.OnAdd(oldObj, false)
	require.Len(t, h.ListEndpoints(), 1)

	assert.NotPanics(t, func() {
		h.OnUpdate(oldObj, newObj)
	})

	warnEntries := logs.FilterLevelExact(zapcore.WarnLevel).All()
	require.NotEmpty(t, warnEntries)
	found := false
	for _, e := range warnEntries {
		if strings.Contains(e.Message, "new object invalid") {
			found = true
			break
		}
	}
	require.True(t, found, "expected warning about invalid new object, got: %#v", warnEntries)
	assert.Empty(t, h.ListEndpoints(), "stale endpoint should be removed when new state is invalid")
}

func TestHandlerOnAddMultipleResourcesOneInvalidOthersStillRegistered(t *testing.T) {
	t.Parallel()

	core, logs := logobserver.New(zapcore.WarnLevel)
	logger := zap.New(core)
	h := &handler{
		idNamespace: "test-1",
		endpoints:   &sync.Map{},
		logger:      logger,
	}

	goodA := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "widget-a",
				"namespace": "ns",
				"uid":       "uid-a",
			},
		},
	}
	goodA.SetUID(types.UID("uid-a"))

	bad := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "widget-bad",
				"namespace": "ns",
			},
		},
	}

	goodC := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "widget-c",
				"namespace": "ns",
				"uid":       "uid-c",
			},
		},
	}
	goodC.SetUID(types.UID("uid-c"))

	assert.NotPanics(t, func() {
		h.OnAdd(goodA, false)
		h.OnAdd(bad, false)
		h.OnAdd(goodC, false)
	})

	endpoints := h.ListEndpoints()
	require.Len(t, endpoints, 2, "valid resources must still be registered when another is incompatible")

	byID := make(map[observer.EndpointID]observer.Endpoint, len(endpoints))
	for _, e := range endpoints {
		byID[e.ID] = e
	}
	require.Contains(t, byID, observer.EndpointID("test-1/uid-a"))
	require.Contains(t, byID, observer.EndpointID("test-1/uid-c"))
	assert.Equal(t, "widget-a", byID[observer.EndpointID("test-1/uid-a")].Target)
	assert.Equal(t, "widget-c", byID[observer.EndpointID("test-1/uid-c")].Target)

	var incompatibleWarns int
	for _, e := range logs.FilterLevelExact(zapcore.WarnLevel).All() {
		if strings.Contains(e.Message, "cannot build observer endpoint") {
			incompatibleWarns++
		}
	}
	require.GreaterOrEqual(t, incompatibleWarns, 1, "expected a warning for the incompatible resource")
}

func TestHandlerOnUpdateOneResourceBecomesInvalidOtherRemainsRegistered(t *testing.T) {
	t.Parallel()

	core, logs := logobserver.New(zapcore.WarnLevel)
	logger := zap.New(core)
	h := &handler{
		idNamespace: "test-1",
		endpoints:   &sync.Map{},
		logger:      logger,
	}

	stable := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "stable",
				"namespace": "ns",
				"uid":       "uid-stable",
			},
		},
	}
	stable.SetUID(types.UID("uid-stable"))

	mutable := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "mutable",
				"namespace": "ns",
				"uid":       "uid-mutable",
			},
		},
	}
	mutable.SetUID(types.UID("uid-mutable"))

	h.OnAdd(stable, false)
	h.OnAdd(mutable, false)
	require.Len(t, h.ListEndpoints(), 2)

	mutableBroken := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "mutable",
				"namespace": "ns",
			},
		},
	}

	assert.NotPanics(t, func() {
		h.OnUpdate(mutable, mutableBroken)
	})

	endpoints := h.ListEndpoints()
	require.Len(t, endpoints, 1)
	assert.Equal(t, observer.EndpointID("test-1/uid-stable"), endpoints[0].ID)
	assert.Equal(t, "stable", endpoints[0].Target)

	found := false
	for _, e := range logs.FilterLevelExact(zapcore.WarnLevel).All() {
		if strings.Contains(e.Message, "new object invalid") {
			found = true
			break
		}
	}
	require.True(t, found, "expected warning when one resource becomes incompatible, got: %#v", logs.All())
}
