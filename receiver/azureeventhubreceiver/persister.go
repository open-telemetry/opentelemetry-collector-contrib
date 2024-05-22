// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"fmt"

	jsoniter "github.com/json-iterator/go"

	"go.opentelemetry.io/collector/extension/experimental/storage"
)

// "github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
// "github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/checkpoints"
// "github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/checkpoints"

const (
	// storageKeyFormat the format of the key used to store the checkpoint in the storage.
	storageKeyFormat = "%s/%s/%s/%s"
	// StartOfStream is a constant defined to represent the start of a partition stream in EventHub.
	StartOfStream = "-1"

	// EndOfStream is a constant defined to represent the current end of a partition stream in EventHub.
	// This can be used as an offset argument in receiver creation to start receiving from the latest
	// event, instead of a specific offset or point in time.
	EndOfStream = "@latest"
)

const ()

// The Checkpoint type is now maintained here to eliminate the dependance on the deprecated eventhub SDK for this datatype.
// Preserving the previously used structure and tags keeps the receiver compatible with existing checkpoints and avoids
// any need to migrate data.
type Checkpoint struct {
	Offset         string `json:"offset"`
	SequenceNumber int64  `json:"sequenceNumber"`
	EnqueueTime    string `json:"enqueueTime"` // ": "0001-01-01T00:00:00Z"
}

type storageCheckpointPersister struct {
	storageClient storage.Client
}

func (s *storageCheckpointPersister) Write(namespace, name, consumerGroup, partitionID string, checkpoint Checkpoint) error {
	b, err := jsoniter.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return s.storageClient.Set(context.Background(), fmt.Sprintf(storageKeyFormat, namespace, name, consumerGroup, partitionID), b)
}

func (s *storageCheckpointPersister) Read(namespace, name, consumerGroup, partitionID string) (Checkpoint, error) {
	var checkpoint Checkpoint
	bytes, err := s.storageClient.Get(context.Background(), fmt.Sprintf(storageKeyFormat, namespace, name, consumerGroup, partitionID))
	if err != nil {
		// error reading checkpoint
		return Checkpoint{}, err
	} else if len(bytes) == 0 {
		// nil or empty checkpoint
		return NewCheckpointFromStartOfStream(), nil
	}
	err = jsoniter.Unmarshal(bytes, &checkpoint)
	return checkpoint, err
}

// NewCheckpointFromStartOfStream returns a checkpoint for the start of the stream
func NewCheckpointFromStartOfStream() Checkpoint {
	return Checkpoint{
		Offset: StartOfStream,
	}
}

// // NewCheckpointFromEndOfStream returns a checkpoint for the end of the stream
// func NewCheckpointFromEndOfStream() Checkpoint {
// 	return Checkpoint{
// 		Offset: EndOfStream,
// 	}
// }
