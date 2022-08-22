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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/experimental/storage"
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

func (r *receiver) setStorageClient(ctx context.Context, host component.Host) error {
	client, err := GetStorageClient(ctx, r.id, component.KindReceiver, host)
	if err != nil {
		return err
	}

	r.storageClient = client
	return nil
}
