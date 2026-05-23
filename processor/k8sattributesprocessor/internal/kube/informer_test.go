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
	informer := newSharedInformer(client.K8s, "testns", labelSelector, fieldSelector)
	assert.NotNil(t, informer)
}

func Test_newSharedNamespaceInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	informer := newNamespaceSharedInformer(client.K8s)
	assert.NotNil(t, informer)
}

func Test_newSharedDeploymentInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	informer := newDeploymentSharedInformer(client.K8s, "ns")
	assert.NotNil(t, informer)
}

func Test_newSharedStatefulSetInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	informer := newStatefulSetSharedInformer(client.K8s, "ns")
	assert.NotNil(t, informer)
}

func Test_newSharedDaemonSetInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	informer := newDaemonSetSharedInformer(client.K8s, "ns")
	assert.NotNil(t, informer)
}

func Test_newSharedJobInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	informer := newJobSharedInformer(client.K8s, "ns")
	assert.NotNil(t, informer)
}

func Test_newKubeSystemSharedInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	informer := newKubeSystemSharedInformer(client.K8s)
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

func Test_namespaceInformerListFunc(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	listFunc := namespaceInformerListFunc(c.K8s)
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

func Test_namespaceInformerWatchFunc(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	watchFunc := namespaceInformerWatchFunc(c.K8s)
	opts := metav1.ListOptions{}
	obj, err := watchFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
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
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	i := NewFakeNamespaceInformer(c.K8s)
	_, err = i.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{}, time.Second)
	assert.NoError(t, err)
	i.HasSynced()
	i.LastSyncResourceVersion()
	store := i.GetStore()
	assert.NoError(t, store.Add(api_v1.Namespace{}))
}

func Test_replicasetListFuncWithSelectors(t *testing.T) {
	mc := newTestMetadataClient()
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	listFunc := replicaSetListFuncWithSelectors(mc, gvr, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := listFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_replicasetWatchFuncWithSelectors(t *testing.T) {
	mc := newTestMetadataClient()
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	watchFunc := replicaSetWatchFuncWithSelectors(mc, gvr, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := watchFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_newReplicaSetsharedInformer(t *testing.T) {
	mc := newTestMetadataClient()
	informer := newReplicaSetSharedInformer(mc, "test-ns")
	if informer == nil {
		t.Fatalf("Expected informer to be non-nil, but got nil. Check logs for details.")
	}
	assert.NotNil(t, informer)
}

func newTestMetadataClient() clientmeta.Interface {
	mc := clientmetafake.NewSimpleMetadataClient(runtime.NewScheme()) // or NewSimpleClientset for older versions
	return mc
}

func Test_deploymentWatchFuncWithSelectors(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	watchFunc := deploymentWatchFuncWithSelectors(c.K8s, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := watchFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_statefulsetListFuncWithSelectors(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	listFunc := statefulsetListFuncWithSelectors(c.K8s, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := listFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_statefulsetWatchFuncWithSelectors(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	watchFunc := statefulsetWatchFuncWithSelectors(c.K8s, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := watchFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_daemonsetListFuncWithSelectors(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	listFunc := daemonsetListFuncWithSelectors(c.K8s, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := listFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_daemonsetWatchFuncWithSelectors(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	watchFunc := daemonsetWatchFuncWithSelectors(c.K8s, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := watchFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_jobListFuncWithSelectors(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	listFunc := jobListFuncWithSelectors(c.K8s, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := listFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_jobWatchFuncWithSelectors(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	watchFunc := jobWatchFuncWithSelectors(c.K8s, "test-ns")
	opts := metav1.ListOptions{}
	obj, err := watchFunc(t.Context(), opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}
