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
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

func GetStorageClient(ctx context.Context, host component.Host, storageID *config.ComponentID, componentID config.ComponentID) (storage.Client, error) {
	if storageID == nil {
		return storage.NewNopClient(), nil
	}

	extension, ok := host.GetExtensions()[*storageID]
	if !ok {
		return nil, fmt.Errorf("storage extension '%s' not found", storageID)
	}

	storageExtension, ok := extension.(storage.Extension)
	if !ok {
		return nil, fmt.Errorf("non-storage extension '%s' found", storageID)
	}

	return storageExtension.GetClient(ctx, component.KindReceiver, componentID, "")

}

func (r *receiver) setStorageClient(ctx context.Context, host component.Host) error {
	client, err := GetStorageClient(ctx, host, r.storageID, r.id)
	if err != nil {
		return err
	}
	r.storageClient = client
	return nil
}
