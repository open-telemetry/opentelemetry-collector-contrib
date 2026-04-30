// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
)

const (
	defaultEndpoint       = "127.0.0.1:9090"
	defaultReadTimeout    = 10 * time.Minute
	defaultLookbackDelta  = 5 * time.Minute
	defaultMaxConnections = 512
	defaultEnabled        = false
)

type Config struct {
	Enabled        *bool                   `mapstructure:"enabled"`
	LookbackDelta  time.Duration           `mapstructure:"lookback_delta"`
	MaxConnections int                     `mapstructure:"max_connections"`
	ServerConfig   confighttp.ServerConfig `mapstructure:"server_config"`
}

// IsEnabled returns whether the API server is enabled. Defaults to false.
func (cfg *Config) IsEnabled() bool {
	if cfg == nil || cfg.Enabled == nil {
		return defaultEnabled
	}
	return *cfg.Enabled
}

// DefaultConfig returns the default configuration for the Prometheus API server.
func DefaultConfig() Config {
	serverConfig := confighttp.NewDefaultServerConfig()
	serverConfig.NetAddr.Transport = confignet.TransportTypeTCP
	serverConfig.NetAddr.Endpoint = defaultEndpoint
	serverConfig.ReadTimeout = defaultReadTimeout

	return Config{
		LookbackDelta:  defaultLookbackDelta,
		MaxConnections: defaultMaxConnections,
		ServerConfig:   serverConfig,
	}
}

func (cfg *Config) ApplyDefaults() {
	if cfg == nil {
		return
	}

	defaultCfg := DefaultConfig()

	if cfg.LookbackDelta == 0 {
		cfg.LookbackDelta = defaultCfg.LookbackDelta
	}

	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = defaultCfg.MaxConnections
	}

	if cfg.ServerConfig.ReadTimeout <= 0 {
		cfg.ServerConfig.ReadTimeout = defaultCfg.ServerConfig.ReadTimeout
	}

	if cfg.ServerConfig.NetAddr.Transport == "" {
		cfg.ServerConfig.NetAddr.Transport = defaultCfg.ServerConfig.NetAddr.Transport
	}

	if cfg.ServerConfig.NetAddr.Endpoint == "" {
		cfg.ServerConfig.NetAddr.Endpoint = defaultCfg.ServerConfig.NetAddr.Endpoint
	}
}

func (cfg *Config) Validate() error {
	if cfg == nil {
		return nil
	}

	if cfg.LookbackDelta < 0 {
		return errors.New("lookback_delta must be non-negative")
	}

	return cfg.ServerConfig.NetAddr.Validate()
}
