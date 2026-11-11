// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	_ "embed"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

//go:embed testdata/openshift/infrastructure-crd.yaml
var infrastructureCRD []byte

var infrastructureGVR = schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1", Resource: "infrastructures"}

// CreateOpenShiftInfrastructure installs a minimal config.openshift.io Infrastructure CRD
// (for clusters that don't serve it, e.g. kind or MicroShift), creates the "cluster"
// object and sets its status. Delete the returned objects with DeleteObjects.
func CreateOpenShiftInfrastructure(t *testing.T, client *K8sClient, status map[string]any) []*unstructured.Unstructured {
	crd, err := CreateObject(client, infrastructureCRD)
	require.NoError(t, err, "failed to create Infrastructure CRD")

	infra := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "config.openshift.io/v1",
		"kind":       "Infrastructure",
		"metadata":   map[string]any{"name": "cluster"},
	}}
	resource := client.DynamicClient.Resource(infrastructureGVR)
	var cr *unstructured.Unstructured
	// The CRD must be established before the API accepts the object.
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		var createErr error
		cr, createErr = resource.Create(t.Context(), infra, metav1.CreateOptions{})
		assert.NoError(tt, createErr)
	}, 30*time.Second, time.Second, "failed to create Infrastructure object")

	patch, err := json.Marshal(map[string]any{"status": status})
	require.NoError(t, err)
	_, err = resource.Patch(t.Context(), cr.GetName(), types.MergePatchType, patch, metav1.PatchOptions{}, "status")
	require.NoError(t, err, "failed to set Infrastructure status")

	// Refresh discovery so DeleteObjects can map the new kind.
	client.Mapper.Reset()
	return []*unstructured.Unstructured{cr, crd}
}
