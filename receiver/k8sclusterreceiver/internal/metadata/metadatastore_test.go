package metadata

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/stretchr/testify/require"
	"slices"
	"testing"
)

func TestStore_GetFromEmptyStore(t *testing.T) {
	store := NewStore()

	get := store.Get(gvk.Job)

	require.Nil(t, get)
}

func TestStore_SetupAndGet(t *testing.T) {
	store := NewStore()

	store.Setup(gvk.Job, "", mockStore{
		testObjs: []any{"obj-1", "obj-2"},
	})
	store.Setup(gvk.Job, "namespace-1", mockStore{
		testObjs: []any{"obj-3", "obj-4"},
	})
	get := store.Get(gvk.Job)

	require.NotNil(t, get)

	require.Len(t, get, 2)
	generalStore := get[""]
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

func (mockStore) Add(obj interface{}) error {
	return nil
}

func (mockStore) Update(obj interface{}) error {
	return nil
}

func (mockStore) Delete(obj interface{}) error {
	return nil
}

func (m mockStore) List() []interface{} {
	return m.testObjs
}

func (mockStore) ListKeys() []string {
	return []string{}
}

func (mockStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (mockStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (mockStore) Replace(i []interface{}, s string) error {
	return nil
}

func (mockStore) Resync() error {
	return nil
}
