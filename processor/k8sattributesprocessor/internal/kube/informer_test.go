// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	informer := newSharedInformer(client, "testns", labelSelector, fieldSelector)
	assert.NotNil(t, informer)
}

func Test_newSharedNamespaceInformer(t *testing.T) {
	client, err := newFakeAPIClientset(k8sconfig.APIConfig{})
	require.NoError(t, err)
	informer := newNamespaceSharedInformer(client)
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
		Labels: []FieldFilter{
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
		Labels: []FieldFilter{
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
	i := NewFakeInformer(c, "ns", nil, nil)
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
	i := NewFakeNamespaceInformer(c)
	_, err = i.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{}, time.Second)
	assert.NoError(t, err)
	i.HasSynced()
	i.LastSyncResourceVersion()
	store := i.GetStore()
	assert.NoError(t, store.Add(api_v1.Namespace{}))
}
