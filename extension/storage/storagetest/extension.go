// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storagetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

var testStorageType component.Type = "test_storage"

// TestStorage is an in memory storage extension designed for testing
type TestStorage struct {
	component.StartFunc
	component.ShutdownFunc
	ID         component.ID
	storageDir string
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*TestStorage)(nil)

func NewStorageID(name string) component.ID {
	return component.NewIDWithName(testStorageType, name)
}

// NewInMemoryStorageExtension creates a TestStorage extension
func NewInMemoryStorageExtension(name string) *TestStorage {
	return &TestStorage{
		ID: NewStorageID(name),
	}
}

// NewFileBackedStorageExtension creates a TestStorage extension
func NewFileBackedStorageExtension(name string, storageDir string) *TestStorage {
	return &TestStorage{
		ID:         NewStorageID(name),
		storageDir: storageDir,
	}
}

// GetClient returns a storage client for an individual component
func (s *TestStorage) GetClient(ctx context.Context, kind component.Kind, ent component.ID, name string) (storage.Client, error) {
	var client *TestClient
	if s.storageDir == "" {
		client = NewInMemoryClient(kind, ent, name)
	} else {
		client = NewFileBackedClient(kind, ent, name, s.storageDir)
	}
	return client, setCreatorID(ctx, client, s.ID)
}

var nonStorageType component.Type = "non_storage"

// NonStorage is useful for testing expected behaviors that involve
// non-storage extensions
type NonStorage struct {
	component.StartFunc
	component.ShutdownFunc
	ID component.ID
}

// Ensure this extension implements the appropriate interface
var _ extension.Extension = (*NonStorage)(nil)

func NewNonStorageID(name string) component.ID {
	return component.NewIDWithName(nonStorageType, name)
}

// NewNonStorageExtension creates a NonStorage extension
func NewNonStorageExtension(name string) *NonStorage {
	return &NonStorage{
		ID: NewNonStorageID(name),
	}
}
