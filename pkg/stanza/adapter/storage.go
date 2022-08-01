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

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/experimental/storage"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

func GetStorageExtension(ctx context.Context, host component.Host, storageID config.ComponentID) (storage.Extension, error) {
	if host == nil {
		return nil, nil
	}

	// Storage explicitly disabled. (e.g. 'storage: false')
	if storageID.String() == "false" {
		return nil, nil
	}

	// Storage explicitly specified.
	if storageID.String() != "" {
		ext, found := host.GetExtensions()[storageID]
		if !found {
			return nil, fmt.Errorf("storage extension not found: %s", storageID.String())
		}
		if se, ok := ext.(storage.Extension); ok {
			return se, nil
		}
		return nil, errors.New("non-storage extension specified")
	}

	// Storage not specified. Automatically select if unambiguous.
	var storageExt storage.Extension
	for _, ext := range host.GetExtensions() {
		if se, ok := ext.(storage.Extension); ok {
			if storageExt != nil {
				return nil, errors.New("ambiguous storage extension")
			}
			storageExt = se
		}
	}

	// Extension will be nil if none was specified
	return storageExt, nil
}

func GetPersister(storageClient storage.Client) operator.Persister {
	return &persister{storageClient}
}

func (r *receiver) setStorage(ctx context.Context, host component.Host) error {
	extension, err := GetStorageExtension(ctx, host, r.storageID)
	if err != nil {
		return err
	}

	if extension == nil {
		r.storageClient = storage.NewNopClient()
		return nil
	}

	client, err := extension.GetClient(ctx, component.KindReceiver, r.id, "")
	if err != nil {
		return err
	}

	r.storageExtension, r.storageClient = extension, client
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
