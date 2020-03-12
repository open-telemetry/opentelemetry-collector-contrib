package redisreceiver

import (
	"github.com/go-redis/redis/v7"
)

type client interface {
	retrieveInfo() (string, error)
	delimiter() string
}

var _ client = (*redisClient)(nil)

type redisClient struct {
	client *redis.Client
}

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
