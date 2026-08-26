// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	clientmeta "k8s.io/client-go/metadata"
	clientmetafake "k8s.io/client-go/metadata/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func Test_newSharedInformer(t *testing.T) {
	labelSelector, fieldSelector, err := selectorsFromFilters(Filters{})
	require.NoError(t, err)
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	informer := newSharedInformer(client.K8s, "testns", labelSelector, fieldSelector, 0)
	assert.NotNil(t, informer)
}

func Test_newSharedNamespaceInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newNamespaceSharedInformer(mc, 0)
	assert.NotNil(t, informer)
}

func Test_newSharedDeploymentInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newDeploymentSharedInformer(mc, "ns", 0)
	assert.NotNil(t, informer)
}

func Test_newSharedStatefulSetInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newStatefulSetSharedInformer(mc, "ns", 0)
	assert.NotNil(t, informer)
}

func Test_newSharedDaemonSetInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newDaemonSetSharedInformer(mc, "ns", 0)
	assert.NotNil(t, informer)
}

func Test_newSharedJobInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newJobSharedInformer(mc, "ns", 0)
	assert.NotNil(t, informer)
}

func Test_newKubeSystemSharedInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newKubeSystemSharedInformer(mc, 0)
	assert.NotNil(t, informer)
}

func Test_newSharedNodeInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newNodeSharedInformer(mc, "", 0)
	assert.NotNil(t, informer)
}

func Test_newSharedNodeInformerWithName(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newNodeSharedInformer(mc, "test-node", 0)
	assert.NotNil(t, informer)
}

func Test_informerListFuncWithSelectors(t *testing.T) {
	ls, fs, err := selectorsFromFilters(Filters{
		Fields: []FieldFilter{
			{
				Key:   "kk1",
				Value: "kv1",
				Op:    selection.Equals,
			},
		},
		Labels: []LabelFilter{
			{
				Key:   "lk1",
				Value: "lv1",
				Op:    selection.NotEquals,
			},
		},
	})
	assert.NoError(t, err)
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	listFunc := informerListFuncWithSelectors(c.K8s, "test-ns", ls, fs)
	opts := metav1.ListOptions{}
	obj, err := listFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_informerWatchFuncWithSelectors(t *testing.T) {
	ls, fs, err := selectorsFromFilters(Filters{
		Fields: []FieldFilter{
			{
				Key:   "kk1",
				Value: "kv1",
				Op:    selection.Equals,
			},
		},
		Labels: []LabelFilter{
			{
				Key:   "lk1",
				Value: "lv1",
				Op:    selection.NotEquals,
			},
		},
	})
	assert.NoError(t, err)
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	watchFunc := informerWatchFuncWithSelectors(c.K8s, "test-ns", ls, fs)
	opts := metav1.ListOptions{}
	obj, err := watchFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_metadataListFunc(t *testing.T) {
	mc := newTestMetadataClient()
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	listFunc := metadataListFunc(mc, gvr, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := listFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_metadataWatchFunc(t *testing.T) {
	mc := newTestMetadataClient()
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	watchFunc := metadataWatchFunc(mc, gvr, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := watchFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_newReplicaSetsharedInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newReplicaSetSharedInformer(mc, "test-ns", 0)
	if informer == nil {
		t.Fatalf("Expected informer to be non-nil, but got nil. Check logs for details.")
	}
	assert.NotNil(t, informer)
}

func newTestMetadataClient() clientmeta.Interface {
	mc := clientmetafake.NewSimpleMetadataClient(runtime.NewScheme())
	return mc
}

func Test_fakeInformer(t *testing.T) {
	// nothing real to test here. just to make coverage happy
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	i := NewFakeInformer(c.K8s, "ns", nil, nil)
	_, err = i.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{}, time.Second)
	assert.NoError(t, err)
	i.HasSynced()
	i.LastSyncResourceVersion()
	store := i.GetStore()
	assert.NoError(t, store.Add(api_v1.Pod{}))
}

func Test_fakeNamespaceInformer(t *testing.T) {
	// nothing real to test here. just to make coverage happy
	mc := newTestMetadataClient()
	i := NewFakeNamespaceInformer(mc)
	_, err := i.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{}, time.Second)
	assert.NoError(t, err)
	i.HasSynced()
	i.LastSyncResourceVersion()
	store := i.GetStore()
	assert.NoError(t, store.Add(api_v1.Namespace{}))
}
