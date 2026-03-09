package watch

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

type checkpointer struct {
	client storage.Client
	logger *zap.Logger
}

const checkpointKeyFormat = "latestResourceVersion/%s"

func newCheckpointer(client storage.Client, logger *zap.Logger) *checkpointer {
	return &checkpointer{
		client: client,
		logger: logger,
	}
}

func (c *checkpointer) GetResourceVersion(ctx context.Context,
	namespace, objectType string) (string, error) {
	if c.client == nil {
		return "", errors.New("storage client is nil")
	}

	checkPointKey := c.getCheckpointKey(namespace, objectType)
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

func (c *checkpointer) SetResourceVersion(
	ctx context.Context,
	namespace, objectType, resourceVersion string) error {
	if c.client == nil {
		return errors.New("storage client is nil")
	}

	key := c.getCheckpointKey(namespace, objectType)
	if key == "" {
		return fmt.Errorf("checkpoint key is empty: %s, %s", namespace, objectType)
	}

	if err := c.client.Set(ctx, key, []byte(resourceVersion)); err != nil {
		return fmt.Errorf("failed to store resourceVersion with key %s: %w", key, err)
	}

	c.logger.Debug("Checkpoint saved",
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
