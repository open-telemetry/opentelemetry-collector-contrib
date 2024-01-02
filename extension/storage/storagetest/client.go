// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

var (
	errClientClosed = errors.New("client closed")
)

type TestClient struct {
	cache    map[string][]byte
	cacheMux sync.Mutex

	kind component.Kind
	id   component.ID
	name string

	storageFile string

	closed bool
}

// NewInMemoryClient creates a storage.Client that functions as a map[string][]byte
// This is useful for tests that do not involve collector restart behavior.
func NewInMemoryClient(kind component.Kind, id component.ID, name string) *TestClient {
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
func NewFileBackedClient(kind component.Kind, id component.ID, name string, storageDir string) *TestClient {
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

const clientCreatorID = "client_creator_id"

func setCreatorID(ctx context.Context, client storage.Client, creatorID component.ID) error {
	return client.Set(ctx, clientCreatorID, []byte(creatorID.String()))
}

// CreatorID is the component.ID of the extension that created the component
func CreatorID(ctx context.Context, client storage.Client) (component.ID, error) {
	idBytes, err := client.Get(ctx, clientCreatorID)
	if err != nil || idBytes == nil {
		return component.ID{}, err
	}

	id := component.ID{}
	err = id.UnmarshalText(idBytes)
	return id, err
}
