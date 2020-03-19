package redisreceiver

import (
	"github.com/go-redis/redis/v7"
)

// Interface for a Redis client. Implementation can be fake or real.
type client interface {
	// retrieves a string of key/value pairs of redis metadata
	retrieveInfo() (string, error)
	// line delimiter
	// redis lines are delimited by \r\n, files (for testing) by \n
	delimiter() string
}

// Wraps a real redis client, implements client interface.
type redisClient struct {
	client *redis.Client
}

var _ client = (*redisClient)(nil)

func newRedisClient(options *redis.Options) client {
	return &redisClient{
		client: redis.NewClient(options),
	}
}

func (c *redisClient) delimiter() string {
	return "\r\n"
}

func (c *redisClient) retrieveInfo() (string, error) {
	return c.client.Info().Result()
}
