// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/redisstorageextension"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

type redisStorage struct {
	cfg    *Config
	logger *zap.Logger
	client *redis.Client
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*redisStorage)(nil)

func newRedisStorage(logger *zap.Logger, config *Config) (extension.Extension, error) {
	return &redisStorage{
		cfg:    config,
		logger: logger,
	}, nil
}

// Start runs cleanup if configured
func (rs *redisStorage) Start(context.Context, component.Host) error {
	c := redis.NewClient(&redis.Options{
		Addr:     rs.cfg.Endpoint,
		Password: string(rs.cfg.Password),
		DB:       rs.cfg.DB,
	})
	rs.client = c
	return nil
}

// Shutdown will close any open databases
func (rs *redisStorage) Shutdown(context.Context) error {
	if rs.client == nil {
		return nil
	}
	return rs.client.Close()
}

type redisClient struct {
	client     *redis.Client
	prefix     string
	expiration time.Duration
}

var _ storage.Client = redisClient{}

func (rc redisClient) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := rc.client.Get(ctx, rc.prefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return b, err
}

func (rc redisClient) Set(ctx context.Context, key string, value []byte) error {
	_, err := rc.client.Set(ctx, rc.prefix+key, value, rc.expiration).Result()
	return err
}

func (rc redisClient) Delete(ctx context.Context, key string) error {
	_, err := rc.client.Del(ctx, rc.prefix+key).Result()
	return err
}

func (rc redisClient) Batch(ctx context.Context, ops ...*storage.Operation) error {
	p := rc.client.Pipeline()
	for _, op := range ops {
		switch op.Type {
		case storage.Delete:
			p.Del(ctx, rc.prefix+op.Key)
		case storage.Get:
			p.Get(ctx, rc.prefix+op.Key)
		case storage.Set:
			p.Set(ctx, rc.prefix+op.Key, op.Value, rc.expiration)
		}
	}
	_, err := p.Exec(ctx)
	return err
}

func (rc redisClient) Close(_ context.Context) error {
	return nil
}

// GetClient returns a storage client for an individual component
func (rs *redisStorage) GetClient(_ context.Context, kind component.Kind, ent component.ID, name string) (storage.Client, error) {
	return redisClient{
		client:     rs.client,
		prefix:     rs.getPrefix(ent, kindString(kind), name),
		expiration: rs.cfg.Expiration,
	}, nil
}

func (rs *redisStorage) getPrefix(ent component.ID, kind, name string) string {
	var prefix string
	if name == "" {
		prefix = fmt.Sprintf("%s_%s_%s", kind, ent.Type(), ent.Name())
	} else {
		prefix = fmt.Sprintf("%s_%s_%s_%s", kind, ent.Type(), ent.Name(), name)
	}

	if rs.cfg.Prefix != "" {
		prefix = fmt.Sprintf("%s_%s", prefix, rs.cfg.Prefix)
	}

	return prefix
}

func kindString(k component.Kind) string {
	switch k {
	case component.KindReceiver:
		return "receiver"
	case component.KindProcessor:
		return "processor"
	case component.KindExporter:
		return "exporter"
	case component.KindExtension:
		return "extension"
	case component.KindConnector:
		return "connector"
	default:
		return "other" // not expected
	}
}
