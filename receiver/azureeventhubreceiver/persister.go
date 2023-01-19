// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"fmt"

	"github.com/Azure/azure-event-hubs-go/v3/persist"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

const (
	storageKeyFormat = "%s/%s/%s/%s"
)

type storageCheckpointPersister struct {
	storageClient storage.Client
}

func (s *storageCheckpointPersister) Write(namespace, name, consumerGroup, partitionID string, checkpoint persist.Checkpoint) error {
	b, err := jsoniter.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return s.storageClient.Set(context.Background(), fmt.Sprintf(storageKeyFormat, namespace, name, consumerGroup, partitionID), b)
}

func (s *storageCheckpointPersister) Read(namespace, name, consumerGroup, partitionID string) (persist.Checkpoint, error) {
	var checkpoint persist.Checkpoint
	bytes, err := s.storageClient.Get(context.Background(), fmt.Sprintf(storageKeyFormat, namespace, name, consumerGroup, partitionID))
	if err != nil {
		return persist.NewCheckpointFromStartOfStream(), err
	}
	if len(bytes) == 0 {
		return persist.NewCheckpointFromStartOfStream(), err
	}
	err = jsoniter.Unmarshal(bytes, &checkpoint)
	return checkpoint, err
}
