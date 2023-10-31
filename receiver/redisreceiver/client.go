// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"github.com/go-redis/redis/v7"
)

// Interface for a Redis client. Implementation can be faked for testing.
type client interface {
	// retrieves a string of key/value pairs of redis metadata
	retrieveInfo() (string, error)
	// line delimiter
	// redis lines are delimited by \r\n, files (for testing) by \n
	delimiter() string
	// close release redis client connection pool
	close() error
}

// Wraps a real Redis client, implements `client` interface.
type redisClient struct {
	client *redis.Client
}

var _ client = (*redisClient)(nil)

// Creates a new real Redis client from the passed-in redis.Options.
func newRedisClient(options *redis.Options) client {
	return &redisClient{
		client: redis.NewClient(options),
	}
}

// Redis strings are CRLF delimited.
func (c *redisClient) delimiter() string {
	return "\r\n"
}

// Retrieve Redis INFO. We retrieve all of the 'sections'.
func (c *redisClient) retrieveInfo() (string, error) {
	return c.client.Info("all").Result()
}

// close client to release connention pool.
func (c *redisClient) close() error {
	return c.client.Close()
}
