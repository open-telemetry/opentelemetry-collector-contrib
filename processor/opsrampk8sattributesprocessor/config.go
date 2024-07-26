package opsrampk8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sattributesprocessor"

import (
	"fmt"
)

// Config defines configuration for k8s attributes processor.
type Config struct {
	RedisHost   string `mapstructure:"redisHost"`
	RedisPort   string `mapstructure:"redisPort"`
	RedisPass   string `mapstructure:"redisPass"`
	ClusterName string `mapstructure:"clusterName"`
	ClusterUid  string `mapstructure:"clusterUid"`
	NodeName    string `mapstructure:"nodeName"`
}

func (cfg *Config) Validate() error {
	if cfg.RedisHost == "" || cfg.RedisPort == "" || cfg.RedisPass == "" {
		return fmt.Errorf("Redis Host, Redis Port and Redis Pass is mandatory")
	}

	if cfg.ClusterName == "" || cfg.ClusterUid == "" {
		return fmt.Errorf("Cluster Name and Cluster Uid is mandatory")
	}

	if cfg.NodeName == "" {
		return fmt.Errorf("Node Name is mandatory")
	}

	return nil
}
