// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

const checkpointKeyFormat = "cloudwatch/%s"

var json = jsoniter.ConfigCompatibleWithStandardLibrary

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

	key := p.getCheckpointKey(logGroupName)
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
		return "", fmt.Errorf("storage client is nil")
	}

	key := p.getCheckpointKey(logGroupName)
	data, err := p.client.Get(ctx, key)
	if err != nil {
		// No checkpoint found, returning a default "start of stream" equivalent
		p.logger.Warn("Checkpoint not found, starting from the beginning",
			zap.String("logGroup", logGroupName),
			zap.Error(err))

		return newCheckpointTimeFromStartOfStream(), nil
	}

	// Handle case where key exists but value is empty
	if len(data) == 0 {
		p.logger.Warn("Checkpoint key exists but value is empty, starting from the beginning",
			zap.String("logGroup", logGroupName))

		return newCheckpointTimeFromStartOfStream(), nil
	}

	var timestamp string
	err = json.Unmarshal(data, &timestamp)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal timestamp: %w", err)
	}

	return timestamp, nil
}

// getCheckpointKey generates a unique storage key
func (p *cloudwatchCheckpointPersister) getCheckpointKey(logGroupName string) string {
	return fmt.Sprintf(checkpointKeyFormat, logGroupName)
}

// newCheckpointTimeFromStartOfStream returns the default timestamp for starting at the beginning as a string
func newCheckpointTimeFromStartOfStream() string {
	// Return the Unix epoch start time as a string in RFC3339 format
	return time.Unix(0, 0).Format(time.RFC3339)
}
