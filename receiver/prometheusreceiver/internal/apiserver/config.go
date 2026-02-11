// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"

import (
	"fmt"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
)

const defaultEndpoint = "0.0.0.0:9090"

type Config struct {
	ServerConfig confighttp.ServerConfig `mapstructure:"server_config"`
}

func (cfg *Config) ApplyDefaults() {
	if cfg == nil {
		return
	}

	if cfg.ServerConfig.NetAddr.Transport == "" {
		cfg.ServerConfig.NetAddr.Transport = confignet.TransportTypeTCP
	}

	if cfg.ServerConfig.NetAddr.Endpoint == "" {
		cfg.ServerConfig.NetAddr.Endpoint = defaultEndpoint
	}
}

func (cfg *Config) Validate() error {
	cfg.ApplyDefaults()

	if err := cfg.ServerConfig.NetAddr.Validate(); err != nil {
		return fmt.Errorf("server_config::netaddr: %w", err)
	}

	return nil
}
