// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
)

type redisStorageClient struct {
	prefix string
	client *redis.Client
	logger *zap.Logger
}

var errRedisNil = "redis: nil"

func newClient(name string, client *redis.Client, logger *zap.Logger) *redisStorageClient {
	return &redisStorageClient{name, client, logger}
}

// Get will retrieve data from storage that corresponds to the specified key
func (c *redisStorageClient) Get(ctx context.Context, key string) ([]byte, error) {
	var result []byte
	err := c.client.Get(ctx, c.getFullKey(key)).Scan(&result)
	if err != nil {
		if err.Error() == errRedisNil {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

// Set will store data. The data can be retrieved using the same key
func (c *redisStorageClient) Set(ctx context.Context, key string, value []byte) error {
	err := c.client.Set(ctx, c.getFullKey(key), value, 0).Err()
	return err
}

// Delete will delete data associated with the specified key
func (c *redisStorageClient) Delete(ctx context.Context, key string) error {
	err := c.client.Del(ctx, c.getFullKey(key)).Err()
	return err
}

// Batch executes the specified operations in order. Get operation results are updated in place
func (c *redisStorageClient) Batch(ctx context.Context, ops ...storage.Operation) error {
	var err error
	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			op.Value, err = c.Get(ctx, op.Key)
		case storage.Set:
			err = c.Set(ctx, op.Key, op.Value)
		case storage.Delete:
			err = c.Delete(ctx, op.Key)
		default:
			return errors.New("wrong operation type")
		}

		if err != nil {
			return err
		}
	}
	return err
}

// Close will close the database
func (c *redisStorageClient) Close(_ context.Context) error {
	return nil
}

func (c *redisStorageClient) getFullKey(key string) string {
	return fmt.Sprintf("%s/%s", c.prefix, key)
}
