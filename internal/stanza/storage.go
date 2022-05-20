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

package stanza // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/experimental/storage"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

func GetStorageClient(ctx context.Context, id config.ComponentID, componentKind component.Kind, host component.Host) (storage.Client, error) {
	var storageExtension storage.Extension
	if host != nil {
		for _, ext := range host.GetExtensions() {
			if se, ok := ext.(storage.Extension); ok {
				if storageExtension != nil {
					return nil, errors.New("multiple storage extensions found")
				}
				storageExtension = se
			}
		}
	}

	if storageExtension == nil {
		return storage.NewNopClient(), nil
	}

	return storageExtension.GetClient(ctx, componentKind, id, "")
}

func GetPersister(storageClient storage.Client) operator.Persister {
	return &persister{storageClient}
}

func (r *receiver) setStorageClient(ctx context.Context, host component.Host) error {
	client, err := GetStorageClient(ctx, r.id, component.KindReceiver, host)
	if err != nil {
		return err
	}

	r.storageClient = client
	return nil
}

func (r *receiver) getPersister() operator.Persister {
	return GetPersister(r.storageClient)
}

type persister struct {
	client storage.Client
}

var _ operator.Persister = &persister{}

func (p *persister) Get(ctx context.Context, key string) ([]byte, error) {
	return p.client.Get(ctx, key)
}

func (p *persister) Set(ctx context.Context, key string, value []byte) error {
	return p.client.Set(ctx, key, value)
}

func (p *persister) Delete(ctx context.Context, key string) error {
	return p.client.Delete(ctx, key)
}
