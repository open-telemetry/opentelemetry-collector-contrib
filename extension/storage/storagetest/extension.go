// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storagetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

var testStorageType config.Type = "test_storage"

// TestStorage is an in memory storage extension designed for testing
type TestStorage struct {
	config.ExtensionSettings
	storageDir string
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*TestStorage)(nil)

func NewStorageID(name string) config.ComponentID {
	return config.NewComponentIDWithName(testStorageType, name)
}

// NewInMemoryStorageExtension creates a TestStorage extension
func NewInMemoryStorageExtension(name string) *TestStorage {
	return &TestStorage{
		ExtensionSettings: config.NewExtensionSettings(
			NewStorageID(name),
		),
	}
}

// NewFileBackedStorageExtension creates a TestStorage extension
func NewFileBackedStorageExtension(name string, storageDir string) *TestStorage {
	return &TestStorage{
		ExtensionSettings: config.NewExtensionSettings(
			NewStorageID(name),
		),
		storageDir: storageDir,
	}
}

// Start does nothing
func (s *TestStorage) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown does nothing
func (s *TestStorage) Shutdown(ctx context.Context) error {
	return nil
}

// GetClient returns a storage client for an individual component
func (s *TestStorage) GetClient(ctx context.Context, kind component.Kind, ent config.ComponentID, name string) (storage.Client, error) {
	var client *TestClient
	if s.storageDir == "" {
		client = NewInMemoryClient(kind, ent, name)
	} else {
		client = NewFileBackedClient(kind, ent, name, s.storageDir)
	}
	return client, setCreatorID(ctx, client, s.ID())
}

var nonStorageType config.Type = "non_storage"

// NonStorage is useful for testing expected behaviors that involve
// non-storage extensions
type NonStorage struct {
	config.ExtensionSettings
}

// Ensure this extension implements the appropriate interface
var _ component.Extension = (*NonStorage)(nil)

func NewNonStorageID(name string) config.ComponentID {
	return config.NewComponentIDWithName(nonStorageType, name)
}

// NewNonStorageExtension creates a NonStorage extension
func NewNonStorageExtension(name string) *NonStorage {
	return &NonStorage{
		ExtensionSettings: config.NewExtensionSettings(
			NewNonStorageID(name),
		),
	}
}

// Start does nothing
func (ns *NonStorage) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown does nothing
func (ns *NonStorage) Shutdown(context.Context) error {
	return nil
}
