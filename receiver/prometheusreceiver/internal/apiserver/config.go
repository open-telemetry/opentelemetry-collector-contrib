// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
)

const (
	defaultEndpoint       = "127.0.0.1:9090"
	defaultReadTimeout    = 10 * time.Minute
	defaultMaxConnections = 512
)

type Config struct {
	Enabled        bool                    `mapstructure:"enabled"`
	MaxConnections int                     `mapstructure:"max_connections"`
	ServerConfig   confighttp.ServerConfig `mapstructure:"server_config"`
}

// DefaultConfig returns the default configuration for the Prometheus API server.
func DefaultConfig() Config {
	serverConfig := confighttp.NewDefaultServerConfig()
	serverConfig.NetAddr.Transport = confignet.TransportTypeTCP
	serverConfig.NetAddr.Endpoint = defaultEndpoint
	serverConfig.ReadTimeout = defaultReadTimeout

	return Config{
		MaxConnections: defaultMaxConnections,
		ServerConfig:   serverConfig,
	}
}

func (cfg *Config) Validate() error {
	return cfg.ServerConfig.NetAddr.Validate()
}
