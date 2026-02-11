// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"

import (
	"errors"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
)

var (
	errTooManySources     = errors.New("too many sources specified, has to be either 'file' or 'remote'")
	errNoSources          = errors.New("no sources specified, has to be either 'file' or 'remote'")
	errAtLeastOneProtocol = errors.New("no protocols selected to serve the strategies, use 'grpc', 'http', or both")
	errSamePortConflict   = errors.New("HTTP and gRPC servers cannot use the same port")
)

// Config has the configuration for the extension enabling the health check
// extension, used to report the health status of the service.
type Config struct {
	HTTPServerConfig *confighttp.ServerConfig `mapstructure:"http"`
	GRPCServerConfig *configgrpc.ServerConfig `mapstructure:"grpc"`

	// Source configures the source for the strategies file. One of `remote` or `file` has to be specified.
	Source Source `mapstructure:"source"`
}

type Source struct {
	// Remote defines the remote location for the file
	Remote *configgrpc.ClientConfig `mapstructure:"remote"`

	// File specifies a local file as the strategies source
	File string `mapstructure:"file"`

	// ReloadInterval determines the periodicity to refresh the strategies
	ReloadInterval time.Duration `mapstructure:"reload_interval"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	// Validate the protocol configuration. At least one protocol should be configured to serve the strategies.
	if cfg.HTTPServerConfig == nil && cfg.GRPCServerConfig == nil {
		return errAtLeastOneProtocol
	}

	// Validate port conflict between HTTP and gRPC servers
	if cfg.HTTPServerConfig != nil && cfg.GRPCServerConfig != nil {
		httpPort, err := extractPort(cfg.HTTPServerConfig.NetAddr.Endpoint)
		if err != nil {
			return fmt.Errorf("invalid HTTP endpoint: %w", err)
		}

		grpcPort, err := extractPort(cfg.GRPCServerConfig.NetAddr.Endpoint)
		if err != nil {
			return fmt.Errorf("invalid gRPC endpoint: %w", err)
		}

		if httpPort == grpcPort {
			return fmt.Errorf("%w: both configured to use port %s. "+
				"Configure different ports for HTTP and gRPC servers. "+
				"Example: http.endpoint='0.0.0.0:5778', grpc.endpoint='0.0.0.0:14250'",
				errSamePortConflict, grpcPort)
		}
	}

	// Validate source configuration
	if cfg.Source.File != "" && cfg.Source.Remote != nil {
		return errTooManySources
	}

	if cfg.Source.File == "" && cfg.Source.Remote == nil {
		return errNoSources
	}

	return nil
}

// extractPort extracts the port from an endpoint string.
func extractPort(endpoint string) (string, error) {
	// Handle empty endpoint
	if endpoint == "" {
		return "", errors.New("endpoint is empty")
	}

	// net.SplitHostPort handles IPv4, IPv6, and various endpoint formats
	_, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		// If SplitHostPort fails, it might be because there is no port specified
		return "", fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
	}

	return port, nil
}
