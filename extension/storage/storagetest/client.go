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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

var (
	errClientClosed = errors.New("client closed")
)

type TestClient struct {
	cache    map[string][]byte
	cacheMux sync.Mutex

	kind component.Kind
	id   config.ComponentID
	name string

	storageFile string

	closed bool
}

// NewInMemoryClient creates a storage.Client that functions as a map[string][]byte
// This is useful for tests that do not involve collector restart behavior.
func NewInMemoryClient(kind component.Kind, id config.ComponentID, name string) *TestClient {
	return &TestClient{
		cache: make(map[string][]byte),
		kind:  kind,
		id:    id,
		name:  name,
	}
}

// NewFileBackedClient creates a storage.Client that will load previous
// storage contents upon creation and save storage contents when closed.
// It also has metadata which may be used to validate test expectations.
func NewFileBackedClient(kind component.Kind, id config.ComponentID, name string, storageDir string) *TestClient {
	client := NewInMemoryClient(kind, id, name)

	client.storageFile = filepath.Join(storageDir, fmt.Sprintf("%d_%s_%s_%s", kind, id.Type(), id.Name(), name))

	// Attempt to load previous storage content
	contents, err := os.ReadFile(client.storageFile)
	if err != nil {
		// Assume no previous storage content
		return client
	}

	previousCache := make(map[string][]byte)
	if err := json.Unmarshal(contents, &previousCache); err != nil {
		// Assume no previous storage content
		return client
	}

	client.cache = previousCache
	return client
}

func (p *TestClient) Get(_ context.Context, key string) ([]byte, error) {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	if p.closed {
		return nil, errClientClosed
	}

	return p.cache[key], nil
}

func (p *TestClient) Set(_ context.Context, key string, value []byte) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	if p.closed {
		return errClientClosed
	}

	p.cache[key] = value
	return nil
}

func (p *TestClient) Delete(_ context.Context, key string) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	if p.closed {
		return errClientClosed
	}

	delete(p.cache, key)
	return nil
}

func (p *TestClient) Batch(_ context.Context, ops ...storage.Operation) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	if p.closed {
		return errClientClosed
	}

	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			op.Value = p.cache[op.Key]
		case storage.Set:
			p.cache[op.Key] = op.Value
		case storage.Delete:
			delete(p.cache, op.Key)
		default:
			return errors.New("wrong operation type")
		}
	}

	return nil
}

func (p *TestClient) Close(_ context.Context) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()

	p.closed = true

	if p.storageFile == "" {
		return nil
	}

	contents, err := json.Marshal(p.cache)
	if err != nil {
		return err
	}

	return os.WriteFile(p.storageFile, contents, os.FileMode(0600))
}

// Kind of component that is using the storage client
func (p *TestClient) Kind() component.Kind {
	return p.kind
}

// ID of component that is using the storage client
func (p *TestClient) ID() config.ComponentID {
	return p.id
}

// Name assigned to the storage client
func (p *TestClient) Name() string {
	return p.name
}
