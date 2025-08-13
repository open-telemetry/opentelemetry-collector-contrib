// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckv2extension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"
)

const (
	httpConfigKey = "http"
	grpcConfigKey = "grpc"
)

var (
	errMissingProtocol      = errors.New("must specify at least one protocol")
	errGRPCEndpointRequired = errors.New("grpc endpoint required")
	errHTTPEndpointRequired = errors.New("http endpoint required")
	errInvalidPath          = errors.New("path must start with /")
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
			return errHTTPEndpointRequired
		}
		if !strings.HasPrefix(c.Path, "/") {
			return errInvalidPath
		}
		return nil
	}

	if c.GRPCConfig == nil && c.HTTPConfig == nil {
		return errMissingProtocol
	}

	if c.HTTPConfig != nil {
		if c.HTTPConfig.Endpoint == "" {
			return errHTTPEndpointRequired
		}
		if c.HTTPConfig.Status.Enabled && !strings.HasPrefix(c.HTTPConfig.Status.Path, "/") {
			return errInvalidPath
		}
		if c.HTTPConfig.Config.Enabled && !strings.HasPrefix(c.HTTPConfig.Config.Path, "/") {
			return errInvalidPath
		}
	}

	if c.GRPCConfig != nil && c.GRPCConfig.NetAddr.Endpoint == "" {
		return errGRPCEndpointRequired
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
