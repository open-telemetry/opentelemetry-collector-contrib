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
	"errors"
	"sync"

	"go.opentelemetry.io/collector/extension/experimental/storage"
)

type MemoryClient struct {
	cache    map[string][]byte
	cacheMux sync.Mutex
}

func NewMemoryClient() *MemoryClient {
	return &MemoryClient{
		cache: make(map[string][]byte),
	}
}

func (p *MemoryClient) Get(_ context.Context, key string) ([]byte, error) {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	return p.cache[key], nil
}

func (p *MemoryClient) Set(_ context.Context, key string, value []byte) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	p.cache[key] = value
	return nil
}

func (p *MemoryClient) Delete(_ context.Context, key string) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	delete(p.cache, key)
	return nil
}

func (p *MemoryClient) Batch(_ context.Context, ops ...storage.Operation) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()

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

func (p *MemoryClient) Close(_ context.Context) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	p.cache = nil
	return nil
}
