// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

const (
	storageKeyFormat = "%s/%s/%s/%s"
)

type storageCheckpointPersister[T any] struct {
	storageClient storage.Client
	defaultValue  T
}

func (s *storageCheckpointPersister[T]) Write(namespace, name, consumerGroup, partitionID string, checkpoint T) error {
	b, err := jsoniter.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return s.storageClient.Set(context.Background(), fmt.Sprintf(storageKeyFormat, namespace, name, consumerGroup, partitionID), b)
}

func (s *storageCheckpointPersister[T]) Read(namespace, name, consumerGroup, partitionID string) (T, error) {
	var checkpoint T
	bytes, err := s.storageClient.Get(context.Background(), fmt.Sprintf(storageKeyFormat, namespace, name, consumerGroup, partitionID))
	if err != nil {
		return s.defaultValue, err
	}
	if len(bytes) == 0 {
		return s.defaultValue, err
	}
	err = jsoniter.Unmarshal(bytes, &checkpoint)
	return checkpoint, err
}
