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
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func Test_newSharedInformer(t *testing.T) {
	labelSelector, fieldSelector, err := selectorsFromFilters(Filters{})
	require.NoError(t, err)
	client := fake.NewClientset()
	stopCh := make(chan struct{})
	informer, err := newSharedInformer(client, "testns", labelSelector, fieldSelector, nil, stopCh)
	assert.NoError(t, err)
	assert.NotNil(t, informer)
	assert.False(t, informer.IsStopped())
	close(stopCh)
	assert.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.True(collect, informer.IsStopped())
	}, time.Second*5, time.Millisecond)
}

func Test_newSharedNamespaceInformer(t *testing.T) {
	client := fake.NewClientset()
	stopCh := make(chan struct{})
	informer, err := newNamespaceSharedInformer(client, nil, stopCh)
	assert.NoError(t, err)
	assert.NotNil(t, informer)
	assert.False(t, informer.IsStopped())
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
		Labels: []FieldFilter{
			{
				Key:   "lk1",
				Value: "lv1",
				Op:    selection.NotEquals,
			},
		},
	})
	assert.NoError(t, err)
	c := fake.NewSimpleClientset()
	listFunc := informerListFuncWithSelectors(c, "test-ns", ls, fs)
	opts := metav1.ListOptions{}
	obj, err := listFunc(opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_namespaceInformerListFunc(t *testing.T) {
	c := fake.NewClientset()
	listFunc := namespaceInformerListFunc(c, nil)
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
		Labels: []FieldFilter{
			{
				Key:   "lk1",
				Value: "lv1",
				Op:    selection.NotEquals,
			},
		},
	})
	assert.NoError(t, err)
	c := fake.NewClientset()
	watchFunc := informerWatchFuncWithSelectors(c, "test-ns", ls, fs)
	opts := metav1.ListOptions{}
	obj, err := watchFunc(opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_namespaceInformerWatchFunc(t *testing.T) {
	c := fake.NewClientset()
	watchFunc := namespaceInformerWatchFunc(c, nil)
	opts := metav1.ListOptions{}
	obj, err := watchFunc(opts)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func Test_fakeInformer(t *testing.T) {
	// nothing real to test here. just to make coverage happy
	stopCh := make(chan struct{})
	i, err := NewFakeInformer("ns", nil, nil, nil, stopCh)
	assert.NoError(t, err)
	_, err = i.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{}, time.Second)
	assert.NoError(t, err)
	i.HasSynced()
	i.LastSyncResourceVersion()
	store := i.GetStore()
	assert.NoError(t, store.Add(api_v1.Pod{}))
}

func Test_fakeNamespaceInformer(t *testing.T) {
	stopCh := make(chan struct{})
	i, err := NewFakeNamespaceInformer(nil, stopCh)
	assert.NoError(t, err)
	_, err = i.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{}, time.Second)
	assert.NoError(t, err)
	i.HasSynced()
	i.LastSyncResourceVersion()
	store := i.GetStore()
	assert.NoError(t, store.Add(api_v1.Namespace{}))
}
