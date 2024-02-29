// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextensionv2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"
)

const (
	httpSettingsKey = "http"
	grpcSettingsKey = "grpc"
)

var (
	errMissingProtocol      = errors.New("healthcheck extension: must be configured for HTTP or gRPC")
	errGRPCEndpointRequired = errors.New("healthcheck extension: grpc endpoint required")
	errHTTPEndpointRequired = errors.New("healthcheck extension: http endpoint required")
	errInvalidPath          = errors.New("healthcheck extension: path must start with /")
)

// Config has the configuration for the extension enabling the health check
// extension, used to report the health status of the service.
type Config struct {
	// LegacySettings contains the settings for the existing healthcheck extension, which we plan
	// to deprecate and remove.
	http.LegacySettings `mapstructure:",squash"`

	// GRPCSettings are v2 settings for the grpc healthcheck service.
	GRPCSettings *grpc.Settings `mapstructure:"grpc"`

	// HTTPSettings are v2 settings for the http healthcheck service.
	HTTPSettings *http.Settings `mapstructure:"http"`

	// ComponentHealthSettings are v2 settings shared between http and grpc services
	ComponentHealthSettings *common.ComponentHealthSettings `mapstructure:"component_health"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (c *Config) Validate() error {
	if !c.UseV2Settings {
		if c.LegacySettings.Endpoint == "" {
			return errHTTPEndpointRequired
		}
		if !strings.HasPrefix(c.LegacySettings.Path, "/") {
			return errInvalidPath
		}
		return nil
	}

	if c.GRPCSettings == nil && c.HTTPSettings == nil {
		return errMissingProtocol
	}

	if c.HTTPSettings != nil {
		if c.HTTPSettings.Endpoint == "" {
			return errHTTPEndpointRequired
		}
		if c.HTTPSettings.Status.Enabled && !strings.HasPrefix(c.HTTPSettings.Status.Path, "/") {
			return errInvalidPath
		}
		if c.HTTPSettings.Config.Enabled && !strings.HasPrefix(c.HTTPSettings.Config.Path, "/") {
			return errInvalidPath
		}
	}

	if c.GRPCSettings != nil && c.GRPCSettings.NetAddr.Endpoint == "" {
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

	if !conf.IsSet(httpSettingsKey) {
		c.HTTPSettings = nil
	}

	if !conf.IsSet(grpcSettingsKey) {
		c.GRPCSettings = nil
	}

	return nil
}
