package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/cache"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Client struct {
	Host                                                 string
	Port                                                 string
	Password                                             string
	GoClient                                             *goredis.Client
	PrimaryCache, SecondaryCache                         *cache.SyncMapWithExpiry
	PrimaryCacheEvictionTime, SecondaryCacheEvictionTime time.Duration
	Enabled                                              bool
	logger                                               *zap.Logger
}

type OpsrampRedisConfig struct {
	RedisHost                  string        `mapstructure:"redisHost"`
	RedisPort                  string        `mapstructure:"redisPort"`
	RedisPass                  string        `mapstructure:"redisPass"`
	ClusterName                string        `mapstructure:"clusterName"`
	ClusterUid                 string        `mapstructure:"clusterUid"`
	NodeName                   string        `mapstructure:"nodeName"`
	PrimaryCacheEvictionTime   time.Duration `mapstructure:"primaryCacheEvictionTime"`
	SecondaryCacheEvictionTime time.Duration `mapstructure:"secondaryCacheEvictionTime"`
}

func NewClient(logger *zap.Logger, primaryCache, secondaryCache *cache.SyncMapWithExpiry, primaryCacheEvictionTime, secondaryCacheEvictionTime time.Duration, rHost, rPort, rPass string) *Client {
	client := Client{
		Host:                       rHost,
		Port:                       rPort,
		Password:                   rPass,
		Enabled:                    true,
		logger:                     logger,
		PrimaryCache:               primaryCache,
		SecondaryCache:             secondaryCache,
		PrimaryCacheEvictionTime:   primaryCacheEvictionTime,
		SecondaryCacheEvictionTime: secondaryCacheEvictionTime,
	}

	if client.Host == "" {
		logger.Info("Redis Host is empty, hence no lookup for moid/resourceuuid cache")
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
		Addr:            c.Host + ":" + c.Port,
		Password:        c.Password,
		MaxRetries:      -1,
		MinRetryBackoff: 55 * time.Millisecond,
		MaxRetryBackoff: 2 * time.Second,
	})

	if err := c.TestConnection(context.Background()); err != nil {
		return err
	}

	return nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	logger := c.logger

	var err error
	err = nil

	for i := 0; i < 15; i++ {
		_, err = c.GoClient.Ping(ctx).Result()
		if err != nil {
			logger.Info("Could not connect/ping to Redis", zap.Any("error", err.Error()))
		} else {
			logger.Info("Connected to Redis")
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		logger.Error("Could not connect/ping to Redis", zap.Any("error", err.Error()))
	}

	return err
}

func (c *Client) GetUuidValueInString(ctx context.Context, key string) string {
	logger := c.logger
	// Check primary cache
	if c.PrimaryCache != nil {
		if val, ok, _, _ := c.PrimaryCache.Load(key); ok {
			logger.Debug("Got value from PrimaryCache", zap.Any("key", key), zap.Any("value", val))
			return val.(string)
		}
	}
	logger.Debug("Failed to fetch the key from primary cache", zap.Any("key", key))
	// Check secondary cache
	if c.SecondaryCache != nil {
		if _, ok, _, _ := c.SecondaryCache.Load(key); ok {
			logger.Debug("Key exists in SecondaryCache", zap.Any("key", key))
			return ""
		}
	}
	logger.Debug("Failed to fetch the key from secondary cache", zap.Any("key", key))
	// Query Redis if enabled
	if c.Enabled {
		val, err := c.GoClient.Get(ctx, key).Result()
		if err == goredis.Nil {
			logger.Debug("Key does not exist in Redis", zap.Any("key", key))
			if c.SecondaryCache != nil {
				c.SecondaryCache.Store(key, "", c.SecondaryCacheEvictionTime, c.SecondaryCacheEvictionTime)
			}
		} else if err != nil {
			logger.Error("Failed to fetch the key from Redis", zap.Error(err))
			if c.SecondaryCache != nil {
				c.SecondaryCache.Store(key, "", c.SecondaryCacheEvictionTime, c.SecondaryCacheEvictionTime)
			}
			return ""
		}

		logger.Debug("Got value from Redis", zap.Any("key", key), zap.Any("value", val))

		// Parse Redis data
		type RedisData struct {
			ResourceUuid string `json:"resourceUuid,omitempty"`
			ResourceHash uint64 `json:"resourceHash,omitempty"`
		}
		var redisData RedisData
		err = json.Unmarshal([]byte(val), &redisData)
		if err != nil {
			logger.Error("Could not unmarshal data", zap.Error(err))
			return ""
		}

		value := redisData.ResourceUuid
		if value == "" && c.SecondaryCache != nil {
			c.SecondaryCache.Store(key, value, c.SecondaryCacheEvictionTime, c.SecondaryCacheEvictionTime)
		} else if c.PrimaryCache != nil {
			c.PrimaryCache.Store(key, value, c.PrimaryCacheEvictionTime, c.PrimaryCacheEvictionTime)
		}
		return value
	}
	return ""
}
