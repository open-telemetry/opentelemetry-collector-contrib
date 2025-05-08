// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

const checkpointKeyFormat = "cloudwatch/%s"

type cloudwatchCheckpointPersister struct {
	client storage.Client
	logger *zap.Logger
}

func newCloudwatchCheckpointPersister(client storage.Client, logger *zap.Logger) *cloudwatchCheckpointPersister {
	return &cloudwatchCheckpointPersister{
		client: client,
		logger: logger,
	}
}

// SetCheckpoint stores the checkpoint (timestamp) for a specific log stream
func (p *cloudwatchCheckpointPersister) SetCheckpoint(
	ctx context.Context,
	logGroupName, timestamp string,
) error {
	if p.client == nil {
		return errors.New("storage client is nil")
	}
	if timestamp == "" {
		return errors.New("timestamp is empty")
	}

	key := p.getCheckpointKey(logGroupName)
	if key == "" {
		return errors.New("checkpoint key is empty")
	}

	data, err := json.Marshal(timestamp)
	if err != nil {
		return fmt.Errorf("failed to marshal timestamp: %w", err)
	}

	if err := p.client.Set(ctx, key, data); err != nil {
		return fmt.Errorf("failed to store checkpoint: %w", err)
	}

	p.logger.Debug("Checkpoint saved",
		zap.String("logGroup", logGroupName),
		zap.String("checkpoint", timestamp))

	return nil
}

// GetCheckpoint retrieves the checkpoint (timestamp) for a specific log stream
func (p *cloudwatchCheckpointPersister) GetCheckpoint(
	ctx context.Context, logGroupName string,
) (string, error) {
	if p.client == nil {
		return "", errors.New("storage client is nil")
	}

	data, err := p.client.Get(ctx, p.getCheckpointKey(logGroupName))
	if err != nil {
		p.logger.Warn("Error retrieving checkpoint",
			zap.String("logGroup", logGroupName),
			zap.Error(err))
		return "", fmt.Errorf("failed to retrieve checkpoint: %w", err)
	}

	// If key is not found, data and error is nil
	if len(data) == 0 {
		p.logger.Debug("No checkpoint found, starting from the beginning",
			zap.String("logGroup", logGroupName))

		return newCheckpointTimeFromStartOfStream(), nil
	}

	var timestamp string
	if err := json.Unmarshal(data, &timestamp); err != nil {
		return "", fmt.Errorf("failed to unmarshal timestamp: %w", err)
	}

	return timestamp, nil
}

// DeleteCheckpoint removes the checkpoint (timestamp) for a specific log stream
func (p *cloudwatchCheckpointPersister) DeleteCheckpoint(
	ctx context.Context, logGroupName string,
) error {
	if p.client == nil {
		return errors.New("storage client is nil")
	}

	if err := p.client.Delete(ctx, p.getCheckpointKey(logGroupName)); err != nil {
		p.logger.Warn("Error deleting checkpoint",
			zap.String("logGroup", logGroupName),
			zap.Error(err))
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	p.logger.Debug("Checkpoint deleted",
		zap.String("logGroup", logGroupName))

	return nil
}

// getCheckpointKey generates a unique storage key
func (p *cloudwatchCheckpointPersister) getCheckpointKey(logGroupName string) string {
	if logGroupName == "" {
		return ""
	}

	return fmt.Sprintf(checkpointKeyFormat, logGroupName)
}

// newCheckpointTimeFromStartOfStream returns the Unix epoch start time as a string in RFC3339 format,
// which is the default timestamp for starting at the beginning.
func newCheckpointTimeFromStartOfStream() string {
	return time.Unix(0, 0).Format(time.RFC3339)
}
