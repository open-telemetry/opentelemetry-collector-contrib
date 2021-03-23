// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"log"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
)

type ObjStore struct {
	sync.RWMutex

	refreshed bool
	objs      map[types.UID]interface{}

	transformFunc func(interface{}) (interface{}, error)
}

func NewObjStore(transformFunc func(interface{}) (interface{}, error)) *ObjStore {
	return &ObjStore{
		transformFunc: transformFunc,
		objs:          map[types.UID]interface{}{},
	}
}

// Track whether the underlying data store is refreshed or not.
// Calling this func itself will reset the state to false.
func (s *ObjStore) Refreshed() bool {
	s.Lock()
	defer s.Unlock()

	refreshed := s.refreshed
	if refreshed {
		s.refreshed = false
	}
	return refreshed
}

func (s *ObjStore) Add(obj interface{}) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		log.Printf("W! Cannot find the metadata for %v.", obj)
		return err
	}

	var toCacheObj interface{}
	if toCacheObj, err = s.transformFunc(obj); err != nil {
		log.Printf("W! Failed to update obj %v in the cached store.", obj)
		return err
	}

	s.Lock()
	defer s.Unlock()

	s.objs[o.GetUID()] = toCacheObj
	s.refreshed = true

	return nil
}

// Update updates the existing entry in the ObjStore.
func (s *ObjStore) Update(obj interface{}) error {
	return s.Add(obj)
}

// Delete deletes an existing entry in the ObjStore.
func (s *ObjStore) Delete(obj interface{}) error {

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	delete(s.objs, o.GetUID())

	s.refreshed = true

	return nil
}

// List implements the List method of the store interface.
func (s *ObjStore) List() []interface{} {
	s.RLock()
	defer s.RUnlock()

	result := make([]interface{}, 0, len(s.objs))
	for _, v := range s.objs {
		result = append(result, v)
	}
	return result
}

// ListKeys implements the ListKeys method of the store interface.
func (s *ObjStore) ListKeys() []string {
	s.RLock()
	defer s.RUnlock()

	result := make([]string, 0, len(s.objs))
	for k := range s.objs {
		result = append(result, string(k))
	}
	return result
}

// Get implements the Get method of the store interface.
func (s *ObjStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// GetByKey implements the GetByKey method of the store interface.
func (s *ObjStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Replace will delete the contents of the store, using instead the
// given list.
func (s *ObjStore) Replace(list []interface{}, _ string) error {
	s.Lock()
	s.objs = map[types.UID]interface{}{}
	s.Unlock()

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
