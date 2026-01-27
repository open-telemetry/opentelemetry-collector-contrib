// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
)

func TestStore_GetFromEmptyStore(t *testing.T) {
	store := NewStore()

	get := store.Get(gvk.Job)

	require.Nil(t, get)
}

func TestStore_SetupAndGet(t *testing.T) {
	store := NewStore()

	store.Setup(gvk.Job, ClusterWideInformerKey, mockStore{
		testObjs: []any{"obj-1", "obj-2"},
	})
	store.Setup(gvk.Job, "namespace-1", mockStore{
		testObjs: []any{"obj-3", "obj-4"},
	})
	get := store.Get(gvk.Job)

	require.NotNil(t, get)

	require.Len(t, get, 2)
	generalStore := get[ClusterWideInformerKey]
	require.NotNil(t, generalStore)

	namespacedStore := get["namespace-1"]
	require.NotNil(t, namespacedStore)

	// make sure the ForEach function visits all items provided by the store

	visitedObjs := []string{}
	store.ForEach(gvk.Job, func(o any) {
		visitedObjs = append(visitedObjs, o.(string))
	})

	slices.Sort(visitedObjs)

	require.Equal(t, []string{"obj-1", "obj-2", "obj-3", "obj-4"}, visitedObjs)
}

type mockStore struct {
	testObjs []any
}

func (mockStore) Add(_ any) error {
	return nil
}

func (mockStore) Update(_ any) error {
	return nil
}

func (mockStore) Delete(_ any) error {
	return nil
}

func (m mockStore) List() []any {
	return m.testObjs
}

func (mockStore) ListKeys() []string {
	return []string{}
}

func (mockStore) Get(_ any) (item any, exists bool, err error) {
	return nil, false, nil
}

func (mockStore) GetByKey(_ string) (item any, exists bool, err error) {
	return nil, false, nil
}

func (mockStore) Replace(_ []any, _ string) error {
	return nil
}

func (mockStore) Resync() error {
	return nil
}
