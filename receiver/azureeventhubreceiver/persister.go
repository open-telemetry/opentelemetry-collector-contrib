// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

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

// The Checkpoint type is now maintained here to eliminate the dependence on the deprecated eventhub SDK for this datatype.
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

// wrappers and stubs to implement azeventhubs.CheckpointStore
func (s *storageCheckpointPersister) ClaimOwnership(_ context.Context, _ []azeventhubs.Ownership, _ *azeventhubs.ClaimOwnershipOptions) ([]azeventhubs.Ownership, error) {
	return nil, nil
}

func (s *storageCheckpointPersister) ListCheckpoints(_ context.Context, _ string, _ string, _ string, _ *azeventhubs.ListCheckpointsOptions) ([]azeventhubs.Checkpoint, error) {
	return nil, nil
}

func (s *storageCheckpointPersister) ListOwnership(_ context.Context, _ string, _ string, _ string, _ *azeventhubs.ListOwnershipOptions) ([]azeventhubs.Ownership, error) {
	return nil, nil
}

func (s *storageCheckpointPersister) SetCheckpoint(_ context.Context, _ azeventhubs.Checkpoint, _ *azeventhubs.SetCheckpointOptions) error {
	return nil
}

var _ azeventhubs.CheckpointStore = &storageCheckpointPersister{}

// NewCheckpointFromStartOfStream returns a checkpoint for the start of the stream
func NewCheckpointFromStartOfStream() Checkpoint {
	return Checkpoint{
		Offset: StartOfStream,
	}
}

func createCheckpointStore(ctx context.Context, host component.Host, cfg *Config, s receiver.CreateSettings) (*storageCheckpointPersister, error) {
	storageClient, err := adapter.GetStorageClient(ctx, host, cfg.StorageID, s.ID)
	if err != nil {
		return nil, err
	}
	return &storageCheckpointPersister{
		storageClient: storageClient,
	}, nil
}
