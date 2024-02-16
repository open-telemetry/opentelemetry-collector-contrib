// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
)

// ObjStore implements the cache.Store interface:
// https://github.com/kubernetes/client-go/blob/release-1.20/tools/cache/store.go#L26-L71
// It is used by cache.Reflector to keep the updated information about resources
// https://github.com/kubernetes/client-go/blob/release-1.20/tools/cache/reflector.go#L48
type ObjStore struct {
	mu sync.RWMutex

	refreshed bool
	objs      map[types.UID]any

	transformFunc func(any) (any, error)
	logger        *zap.Logger
}

func NewObjStore(transformFunc func(any) (any, error), logger *zap.Logger) *ObjStore {
	return &ObjStore{
		transformFunc: transformFunc,
		objs:          map[types.UID]any{},
		logger:        logger,
	}
}

// GetResetRefreshStatus tracks whether the underlying data store is refreshed or not.
// Calling this func itself will reset the state to false.
func (s *ObjStore) GetResetRefreshStatus() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	refreshed := s.refreshed
	if refreshed {
		s.refreshed = false
	}
	return refreshed
}

// Add implements the Add method of the store interface.
// Add adds an entry to the ObjStore.
func (s *ObjStore) Add(obj any) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Cannot find the metadata for %v.", obj))
		return err
	}

	var toCacheObj any
	if toCacheObj, err = s.transformFunc(obj); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to update obj %v in the cached store.", obj))
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.objs[o.GetUID()] = toCacheObj
	s.refreshed = true

	return nil
}

// Update implements the Update method of the store interface.
// Update updates the existing entry in the ObjStore.
func (s *ObjStore) Update(obj any) error {
	return s.Add(obj)
}

// Delete implements the Delete method of the store interface.
// Delete deletes an existing entry in the ObjStore.
func (s *ObjStore) Delete(obj any) error {

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.objs, o.GetUID())

	s.refreshed = true

	return nil
}

// List implements the List method of the store interface.
// List lists all the objects in the ObjStore
func (s *ObjStore) List() []any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]any, 0, len(s.objs))
	for _, v := range s.objs {
		result = append(result, v)
	}
	return result
}

// ListKeys implements the ListKeys method of the store interface.
// ListKeys lists the keys for all objects in the ObjStore
func (s *ObjStore) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.objs))
	for k := range s.objs {
		result = append(result, string(k))
	}
	return result
}

// Get implements the Get method of the store interface.
func (s *ObjStore) Get(_ any) (item any, exists bool, err error) {
	return nil, false, nil
}

// GetByKey implements the GetByKey method of the store interface.
func (s *ObjStore) GetByKey(_ string) (item any, exists bool, err error) {
	return nil, false, nil
}

// Replace implements the Replace method of the store interface.
// Replace will delete the contents of the store, using instead the given list.
func (s *ObjStore) Replace(list []any, _ string) error {
	s.mu.Lock()
	s.objs = map[types.UID]any{}
	s.mu.Unlock()

	for _, o := range list {
		err := s.Add(o)
		if err != nil {
			return err
		}
	}

	return nil
}

// Resync implements the Resync method of the store interface.
func (s *ObjStore) Resync() error {
	return nil
}
