// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
