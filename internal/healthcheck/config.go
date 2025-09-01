// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheck // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/http"
)

// Type aliases to expose internal types publicly
type (
	HTTPLegacyConfig             = http.LegacyConfig
	HTTPConfig                   = http.Config
	PathConfig                   = http.PathConfig
	GRPCConfig                   = grpc.Config
	ComponentHealthConfig        = common.ComponentHealthConfig
	CheckCollectorPipelineConfig = http.CheckCollectorPipelineConfig
)

const (
	httpConfigKey   = "http"
	grpcConfigKey   = "grpc"
	DefaultGRPCPort = 13132
	DefaultHTTPPort = 13133
)

var (
	ErrMissingProtocol      = errors.New("must specify at least one protocol")
	ErrGRPCEndpointRequired = errors.New("grpc endpoint required")
	ErrHTTPEndpointRequired = errors.New("http endpoint required")
	ErrInvalidPath          = errors.New("path must start with /")
)

// Config has the configuration for the extension enabling the health check
// extension, used to report the health status of the service.
type Config struct {
	// LegacyConfig contains the config for the existing healthcheck extension.
	http.LegacyConfig `mapstructure:",squash"`

	// GRPCConfig is v2 config for the grpc healthcheck service.
	GRPCConfig *grpc.Config `mapstructure:"grpc"`

	// HTTPConfig is v2 config for the http healthcheck service.
	HTTPConfig *http.Config `mapstructure:"http"`

	// ComponentHealthConfig is v2 config shared between http and grpc services
	ComponentHealthConfig *common.ComponentHealthConfig `mapstructure:"component_health"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (c *Config) Validate() error {
	if !c.UseV2 {
		if c.Endpoint == "" {
			return ErrHTTPEndpointRequired
		}
		if !strings.HasPrefix(c.Path, "/") {
			return ErrInvalidPath
		}
		return nil
	}

	if c.GRPCConfig == nil && c.HTTPConfig == nil {
		return ErrMissingProtocol
	}

	if c.HTTPConfig != nil {
		if c.HTTPConfig.Endpoint == "" {
			return ErrHTTPEndpointRequired
		}
		if c.HTTPConfig.Status.Enabled && !strings.HasPrefix(c.HTTPConfig.Status.Path, "/") {
			return ErrInvalidPath
		}
		if c.HTTPConfig.Config.Enabled && !strings.HasPrefix(c.HTTPConfig.Config.Path, "/") {
			return ErrInvalidPath
		}
	}

	if c.GRPCConfig != nil && c.GRPCConfig.NetAddr.Endpoint == "" {
		return ErrGRPCEndpointRequired
	}

	return nil
}

// Unmarshal a confmap.Conf into the config struct.
func (c *Config) Unmarshal(conf *confmap.Conf) error {
	err := conf.Unmarshal(c)
	if err != nil {
		return err
	}

	if !conf.IsSet(httpConfigKey) {
		c.HTTPConfig = nil
	}

	if !conf.IsSet(grpcConfigKey) {
		c.GRPCConfig = nil
	}

	return nil
}

func NewDefaultConfig() component.Config {
	return &Config{
		LegacyConfig: http.LegacyConfig{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: testutil.EndpointForPort(DefaultHTTPPort),
			},
			Path: "/",
		},
		HTTPConfig: &http.Config{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: testutil.EndpointForPort(DefaultHTTPPort),
			},
			Status: http.PathConfig{
				Enabled: true,
				Path:    "/status",
			},
			Config: http.PathConfig{
				Enabled: false,
				Path:    "/config",
			},
		},
		GRPCConfig: &grpc.Config{
			ServerConfig: configgrpc.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Endpoint:  testutil.EndpointForPort(DefaultGRPCPort),
					Transport: "tcp",
				},
			},
		},
	}
}
