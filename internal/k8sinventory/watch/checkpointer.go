// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package watch // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/watch"

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

type checkpointer struct {
	client storage.Client
	logger *zap.Logger

	// pending holds the latest resourceVersion per storage key, buffered in
	// memory across all watch streams. Flush() drains it to persistent storage.
	mu      sync.Mutex
	pending map[string]string
}

const checkpointKeyFormat = "latestResourceVersion/%s"

func newCheckpointer(client storage.Client, logger *zap.Logger) *checkpointer {
	return &checkpointer{
		client:  client,
		logger:  logger,
		pending: make(map[string]string),
	}
}

func (c *checkpointer) GetCheckpoint(ctx context.Context, namespace, objectType string) (string, error) {
	if c.client == nil {
		return "", errors.New("storage client is nil")
	}

	checkPointKey := c.getCheckpointKey(namespace, objectType)
	c.logger.Debug("Retrieving checkpoint, key: "+checkPointKey,
		zap.String("namespace", namespace),
		zap.String("objectType", objectType))
	data, err := c.client.Get(ctx, checkPointKey)
	if err != nil {
		c.logger.Warn("Error retrieving checkpoint",
			zap.String("namespace", namespace),
			zap.String("objectType", objectType),
			zap.Error(err))
		return "", fmt.Errorf("failed to retrieve checkpoint: %w", err)
	}

	// If key is not found, data and error is nil
	if len(data) == 0 {
		c.logger.Debug("No checkpoint found, starting from the beginning",
			zap.String("key", checkPointKey))
		return "", nil
	}
	return string(data), nil
}

// SetCheckpoint buffers the latest resourceVersion for the given namespace and
// objectType in memory. Call Flush to persist all buffered values to storage.
// Only updates the in-memory value if the new resourceVersion is numerically
// greater than the current one, acting as a high-watermark. This guards against
// out-of-order resourceVersions from List() responses (which are ordered by
// object key, not by resourceVersion).
func (c *checkpointer) SetCheckpoint(
	_ context.Context,
	namespace, objectType, resourceVersion string,
) error {
	key := c.getCheckpointKey(namespace, objectType)
	if key == "" {
		return fmt.Errorf("checkpoint key is empty: %s, %s", namespace, objectType)
	}

	newRV, err := strconv.ParseInt(resourceVersion, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid resourceVersion %q: %w", resourceVersion, err)
	}

	c.mu.Lock()
	if existing, ok := c.pending[key]; ok {
		if existingRV, err := strconv.ParseInt(existing, 10, 64); err == nil && newRV <= existingRV {
			c.mu.Unlock()
			return nil
		}
	}
	c.pending[key] = resourceVersion
	c.mu.Unlock()

	c.logger.Debug("buffered resourceVersion checkpoint",
		zap.String("key", key),
		zap.String("resourceVersion", resourceVersion))

	return nil
}

// Flush writes all buffered checkpoints to persistent storage. Only the latest
// value per key is written, discarding any intermediate updates since the last
// flush. It is safe to call concurrently from multiple goroutines.
func (c *checkpointer) Flush(ctx context.Context) error {
	if c.client == nil {
		return errors.New("storage client is nil")
	}

	c.mu.Lock()
	if len(c.pending) == 0 {
		c.mu.Unlock()
		return nil
	}
	snapshot := c.pending
	// Setting c.pending to an empty map to avoid unnecessary writes when there are no pending updates
	// to be flushed to the disk.
	c.pending = make(map[string]string)
	c.mu.Unlock()

	failed := false
	for key, rv := range snapshot {
		if err := c.client.Set(ctx, key, []byte(rv)); err != nil {
			c.logger.Error("failed to flush checkpoint",
				zap.String("key", key),
				zap.String("resourceVersion", rv),
				zap.Error(err))
			failed = true
			continue
		}
		c.logger.Debug("flushed resourceVersion checkpoint",
			zap.String("key", key),
			zap.String("resourceVersion", rv))
	}
	if failed {
		return errors.New("one or more checkpoints failed to be stored")
	}
	return nil
}

// DeleteCheckpoint deletes the persisted checkpoint for a given namespace and object type.
// This is used when the persisted resourceVersion is no longer valid (e.g., after a 410 Gone error).
func (c *checkpointer) DeleteCheckpoint(
	ctx context.Context,
	namespace, objectType string,
) error {
	if c.client == nil {
		return errors.New("storage client is nil")
	}

	key := c.getCheckpointKey(namespace, objectType)
	if key == "" {
		return fmt.Errorf("checkpoint key is empty: %s, %s", namespace, objectType)
	}

	if err := c.client.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete resourceVersion with key %s: %w", key, err)
	}

	c.logger.Debug("Checkpoint deleted with key: "+key,
		zap.String("namespace", namespace),
		zap.String("objectType", objectType))

	return nil
}

// getCheckpointKey generates a unique storage key
// returns resourceVersion key for global watch stream (without namespace) or
// per namespace watch stream.
func (*checkpointer) getCheckpointKey(namespace, objectType string) string {
	// when watch stream is cluster-wide or cluster-scoped resource (no namespace),
	// the resource version is persisted per object type only.
	if namespace == "" {
		// example: latestResourceVersion/nodes, latestResourceVersion/namespaces
		return fmt.Sprintf(checkpointKeyFormat, objectType)
	}

	// when watch stream is created per namespace, the resource version is persisted
	// per object type per namespace.
	// example: latestResourceVersion/pods.default, latestResourceVersion/configmaps.kube-system
	return fmt.Sprintf("%s.%s", fmt.Sprintf(checkpointKeyFormat, objectType), namespace)
}
