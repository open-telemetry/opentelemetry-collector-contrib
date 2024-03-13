// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/redisstorage"

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
)

type redisStorage struct {
	address  string
	username string
	password string
	client   *redis.Client
	logger   *zap.Logger
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*redisStorage)(nil)

func newRedisStorage(logger *zap.Logger, config *Config) (extension.Extension, error) {
	return &redisStorage{
		address:  config.Address,
		username: config.Username,
		password: config.Password,
		logger:   logger,
	}, nil
}

// Start opens a connection to the database
func (rs *redisStorage) Start(context.Context, component.Host) error {
	client := redis.NewClient(&redis.Options{
		Addr:     rs.address,
		Username: rs.username,
		Password: rs.password,
		DB:       0, // use default DB
	})
	rs.client = client
	return nil
}

// Shutdown closes the connection to the database
func (rs *redisStorage) Shutdown(context.Context) error {
	if rs.client == nil {
		return nil
	}
	return rs.client.Close()
}

// GetClient returns a storage client for an individual component
func (rs *redisStorage) GetClient(_ context.Context, kind component.Kind, ent component.ID, name string) (storage.Client, error) {
	var fullName string
	if name == "" {
		fullName = fmt.Sprintf("%s/%s/%s", kindString(kind), ent.Type(), ent.Name())
	} else {
		fullName = fmt.Sprintf("%s/%s/%s/%s", kindString(kind), ent.Type(), ent.Name(), name)
	}
	fullName = strings.ReplaceAll(fullName, " ", "")
	return newClient(fullName, rs.client, rs.logger), nil
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
