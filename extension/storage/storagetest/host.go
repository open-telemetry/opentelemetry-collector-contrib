// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storagetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
)

type StorageHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func NewStorageHost() *StorageHost {
	return &StorageHost{
		Host:       componenttest.NewNopHost(),
		extensions: make(map[component.ID]component.Component),
	}
}

func (h *StorageHost) WithExtension(id component.ID, ext extension.Extension) *StorageHost {
	h.extensions[id] = ext
	return h
}

func (h *StorageHost) WithInMemoryStorageExtension(name string) *StorageHost {
	ext := NewInMemoryStorageExtension(name)
	h.extensions[ext.ID] = ext
	return h
}

func (h *StorageHost) WithFileBackedStorageExtension(name, storageDir string) *StorageHost {
	ext := NewFileBackedStorageExtension(name, storageDir)
	h.extensions[ext.ID] = ext
	return h
}

func (h *StorageHost) WithNonStorageExtension(name string) *StorageHost {
	ext := NewNonStorageExtension(name)
	h.extensions[ext.ID] = ext
	return h
}

func (h *StorageHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}
