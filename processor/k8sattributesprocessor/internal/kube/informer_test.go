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
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func Test_newSharedInformer(t *testing.T) {
	labelSelector, fieldSelector, err := selectorsFromFilters(Filters{})
	require.NoError(t, err)
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	stopCh := make(chan struct{})
	informer, err := newSharedInformer(client, "testns", labelSelector, fieldSelector, nil, stopCh)
	require.NoError(t, err)
	assert.NotNil(t, informer)
	assert.False(t, informer.IsStopped())
	close(stopCh)
	assert.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.True(collect, informer.IsStopped())
	}, time.Second*5, time.Millisecond)
}

func Test_newSharedNamespaceInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	stopCh := make(chan struct{})
	informer := newNamespaceSharedInformer(client, stopCh)
	assert.NotNil(t, informer)
	assert.False(t, informer.IsStopped())
	close(stopCh)
	assert.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.True(collect, informer.IsStopped())
	}, time.Second*5, time.Millisecond)
}

func Test_newSharedDeploymentInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	stopCh := make(chan struct{})
	informer, err := newDeploymentSharedInformer(client, "ns", nil, stopCh)
	require.NoError(t, err)
	assert.NotNil(t, informer)
}

func Test_newSharedStatefulSetInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	stopCh := make(chan struct{})
	informer, err := newStatefulSetSharedInformer(client, "ns", nil, stopCh)
	require.NoError(t, err)
	assert.NotNil(t, informer)
}

func Test_newKubeSystemSharedInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	stopCh := make(chan struct{})
	informer := newKubeSystemSharedInformer(client, stopCh)
	assert.NotNil(t, informer)
	close(stopCh)
	assert.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.True(collect, informer.IsStopped())
	}, time.Second*5, time.Millisecond)
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
	listFunc := informerListFuncWithSelectors(c, "test-ns", ls, fs)
	opts := metav1.ListOptions{}
	obj, err := listFunc(opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_namespaceInformerListFunc(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	listFunc := namespaceInformerListFunc(c)
	opts := metav1.ListOptions{}
	obj, err := listFunc(opts)
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
	watchFunc := informerWatchFuncWithSelectors(c, "test-ns", ls, fs)
	opts := metav1.ListOptions{}
	obj, err := watchFunc(opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_namespaceInformerWatchFunc(t *testing.T) {
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	watchFunc := namespaceInformerWatchFunc(c)
	opts := metav1.ListOptions{}
	obj, err := watchFunc(opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_fakeInformer(t *testing.T) {
	// nothing real to test here. just to make coverage happy
	c, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	stopCh := make(chan struct{})
	i, err := NewFakeInformer(c, "ns", nil, nil, nil, stopCh)
	require.NoError(t, err)
	require.NotNil(t, i)
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
	stopCh := make(chan struct{})
	i := NewFakeNamespaceInformer(c, stopCh)
	_, err = i.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{}, time.Second)
	assert.NoError(t, err)
	i.HasSynced()
	i.LastSyncResourceVersion()
	store := i.GetStore()
	assert.NoError(t, store.Add(api_v1.Namespace{}))
}
