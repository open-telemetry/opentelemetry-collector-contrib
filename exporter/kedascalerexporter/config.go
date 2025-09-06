// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kedascalerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kedascalerexporter"

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
)

const (
	defaultScalerGRPCEndpoint     = "localhost:4318"
	defaultMonitoringHTTPEndpoint = "localhost:9090"
	defaultCleanupInterval        = 30 * time.Second
)

type EngineSettings struct {
	// Timeout specifies the maximum duration for processing PromQL queries
	Timeout time.Duration `mapstructure:"timeout"`
	// MaxSamples defines the maximum number of samples that can be processed
	MaxSamples int `mapstructure:"max_samples"`
	// LookbackDelta determines how far back in time to look for data points
	LookbackDelta time.Duration `mapstructure:"lookback_delta"`
	// RetentionDuration specifies how long to keep the data in memory
	RetentionDuration time.Duration `mapstructure:"retention_duration"`
}

type Config struct {
	// Scaler contains the gRPC server configuration for the scaler service
	Scaler *configgrpc.ServerConfig `mapstructure:"scaler_grpc"`
	// MonitoringHTTP contains the HTTP server configuration for monitoring endpoints
	MonitoringHTTP *confighttp.ServerConfig `mapstructure:"monitoring_http"`
	// EngineSettings contains the configuration for the engine's behavior and limits
	EngineSettings *EngineSettings `mapstructure:"engine_settings"`
	// CleanupInterval specifies the interval for cleaning up old data
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

func checkPortFromEndpoint(endpoint string) error {
	_, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is not formatted correctly: %w", err)
	}
	port, err := strconv.ParseInt(portStr, 10, 0)
	if err != nil {
		return fmt.Errorf("endpoint port is not a number: %w", err)
	}
	if port < 1 || port > 65535 {
		return errors.New("port number must be between 1 and 65535")
	}
	return nil
}

func (cfg *Config) Validate() error {
	if cfg.Scaler == nil {
		return errors.New("scaler_grpc config is required")
	}
	if cfg.Scaler != nil {
		if err := checkPortFromEndpoint(cfg.Scaler.NetAddr.Endpoint); err != nil {
			return fmt.Errorf(
				"invalid port number for the gRPC endpoint: %s err: %w",
				cfg.Scaler.NetAddr.Endpoint,
				err,
			)
		}
	}
	if cfg.EngineSettings == nil {
		return errors.New("engine_settings config is required")
	}
	if cfg.EngineSettings.RetentionDuration < 1*time.Minute {
		return errors.New("retention_duration must be at least 1 minute")
	}
	if cfg.CleanupInterval <= 0 {
		return errors.New("cleanup_interval must be greater than 0")
	}
	return nil
}

func CreateDefaultConfig() component.Config {
	return &Config{
		EngineSettings: &EngineSettings{
			Timeout:           30 * time.Second,
			MaxSamples:        500000,
			LookbackDelta:     5 * time.Minute,
			RetentionDuration: 2 * time.Minute,
		},
		Scaler: &configgrpc.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint:  defaultScalerGRPCEndpoint,
				Transport: confignet.TransportTypeTCP,
			},
		},
		MonitoringHTTP: &confighttp.ServerConfig{
			Endpoint: defaultMonitoringHTTPEndpoint,
		},
		CleanupInterval: defaultCleanupInterval,
	}
}
