package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Client struct {
	Host      string
	Port      string
	Password  string
	GoClient  *goredis.Client
	Enabled   bool
	Connected bool
	logger    *zap.Logger
}

func NewClient(logger *zap.Logger, rHost, rPort, rPass string) *Client {
	client := Client{
		Host:      rHost,
		Port:      rPort,
		Password:  rPass,
		Enabled:   true,
		Connected: false,
		logger:    logger,
	}

	if client.Host == "" {
		client.Enabled = false
		return &client
	}

	if client.Port == "" {
		client.Port = "6379"
	}

	client.Init()

	return &client
}

func (c *Client) Init() error {
	c.GoClient = goredis.NewClient(&goredis.Options{
		Addr:     c.Host + ":" + c.Port,
		Password: c.Password,
		DB:       0,
	})

	if err := c.TestConnection(context.Background()); err != nil {
		c.Connected = false
		return err
	}

	c.Connected = true

	return nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	logger := c.logger
	_, err := c.GoClient.Ping(ctx).Result()
	if err != nil {
		logger.Error("Could not connect/ping to Redis", zap.Any("error", err.Error()))
		return err
	}
	logger.Info("Connected to Redis")

	return nil
}

func (c *Client) GetValueInString(ctx context.Context, key string) (value string) {

	logger := c.logger

	if c.Enabled {
		if c.Connected {
			val, err := c.GoClient.Get(ctx, key).Result()
			if err == goredis.Nil {
				logger.Debug("key does not exist", zap.Any("key", key))
			} else if err != nil {
				logger.Info("Trying to reconnect")
				c.Init()
			} else {
				value = val
			}
		} else {
			logger.Info("Trying to reconnect")
			c.Init()
		}
	}
	return
}
