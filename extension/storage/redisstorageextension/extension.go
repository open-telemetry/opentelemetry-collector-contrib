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
func (rs *redisStorage) Start(ctx context.Context, _ component.Host) error {
	tlsConfig, err := rs.cfg.TLS.LoadTLSConfig(ctx)
	if err != nil {
		return err
	}
	c := redis.NewClient(&redis.Options{
		Addr:      rs.cfg.Endpoint,
		Password:  string(rs.cfg.Password),
		DB:        rs.cfg.DB,
		TLSConfig: tlsConfig,
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
	logger     *zap.Logger
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
		case storage.Set:
			p.Set(ctx, rc.prefix+op.Key, op.Value, rc.expiration)
		}
	}
	_, err := p.Exec(ctx)
	if err != nil {
		return err
	}
	// once the pipeline has been executed, we need to fetch all the values
	// and set them on the op
	for _, op := range ops {
		if op.Type == storage.Get {
			value, e := rc.client.Get(ctx, rc.prefix+op.Key).Bytes()
			if e != nil {
				if errors.Is(e, redis.Nil) {
					continue
				}
				return e
			}
			if value != nil {
				// the output of Bucket.Get is only valid within a transaction, so we need to make a copy
				// to be able to return the value
				op.Value = make([]byte, len(value))
				copy(op.Value, value)
			} else {
				op.Value = nil
			}
		}
	}
	return err
}

// IncrementBy atomically adds delta to key and returns the new value.
func (rc redisClient) IncrementBy(ctx context.Context, key string, delta int64) (int64, error) {
	res, err := rc.BatchIncrementBy(ctx, map[string]int64{key: delta})
	if err != nil {
		return 0, err
	}
	return res[key], nil
}

// BatchIncrementBy atomically adds each delta to its key and returns the new values.
// On error some increments may already be applied, so it is not safe to retry.
func (rc redisClient) BatchIncrementBy(ctx context.Context, deltas map[string]int64) (map[string]int64, error) {
	out := make(map[string]int64, len(deltas))
	if len(deltas) == 0 {
		return out, nil
	}

	p := rc.client.Pipeline()
	incrs := make(map[string]*redis.IntCmd, len(deltas))
	ttls := make(map[string]*redis.DurationCmd, len(deltas))
	for k, d := range deltas {
		incrs[k] = p.IncrBy(ctx, rc.prefix+k, d)
		if rc.expiration > 0 {
			ttls[k] = p.TTL(ctx, rc.prefix+k)
		}
	}
	if _, err := p.Exec(ctx); err != nil {
		return nil, err
	}
	for k, c := range incrs {
		out[k] = c.Val()
	}

	// TTL is -1 only for keys just created by INCRBY, avoids EXPIRE NX (Redis 7.0+).
	var created []string
	for k, c := range ttls {
		if c.Val() == -1 {
			created = append(created, k)
		}
	}
	if len(created) > 0 {
		ep := rc.client.Pipeline()
		for _, k := range created {
			ep.Expire(ctx, rc.prefix+k, rc.expiration)
		}
		if _, err := ep.Exec(ctx); err != nil {
			// Not returned, a retry would double count.
			rc.logger.Warn("failed to set expiration on new counter keys",
				zap.Int("keys", len(created)), zap.Error(err))
		}
	}
	return out, nil
}

func (redisClient) Close(context.Context) error {
	return nil
}

// GetClient returns a storage client for an individual component
func (rs *redisStorage) GetClient(_ context.Context, kind component.Kind, ent component.ID, name string) (storage.Client, error) {
	return redisClient{
		client:     rs.client,
		prefix:     rs.getPrefix(ent, kindString(kind), name),
		expiration: rs.cfg.Expiration,
		logger:     rs.logger,
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
