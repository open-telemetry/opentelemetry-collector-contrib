// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpgmireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver"

import (
	"encoding"
	"errors"
	"fmt"
	"net/url"
	"path"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
)

type SanitizedURLPath string

var _ encoding.TextUnmarshaler = (*SanitizedURLPath)(nil)

func (s *SanitizedURLPath) UnmarshalText(text []byte) error {
	u, err := url.Parse(string(text))
	if err != nil {
		return fmt.Errorf("invalid HTTP URL path set for signal: %w", err)
	}

	if !path.IsAbs(u.Path) {
		u.Path = "/" + u.Path
	}

	*s = SanitizedURLPath(u.Path)
	return nil
}

// LicenseConfig defines configuration for license validation
type LicenseConfig struct {
	// ValidationURL is the URL for license validation
	// The receiver will send GET request to this URL with licenseKey as query parameter
	// Expected response: 200 OK for valid license, 4xx/5xx for invalid license
	ValidationURL string `mapstructure:"validation_url"`
	// CacheTimeout is the timeout for license cache in seconds (default: 3600)
	CacheTimeout int `mapstructure:"cache_timeout"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type HTTPConfig struct {
	ServerConfig confighttp.ServerConfig `mapstructure:",squash"`

	// The URL path to receive traces on. If omitted "/v1/traces" will be used.
	TracesURLPath SanitizedURLPath `mapstructure:"traces_url_path,omitempty"`

	// The URL path to receive metrics on. If omitted "/v1/metrics" will be used.
	MetricsURLPath SanitizedURLPath `mapstructure:"metrics_url_path,omitempty"`

	// The URL path to receive logs on. If omitted "/v1/logs" will be used.
	LogsURLPath SanitizedURLPath `mapstructure:"logs_url_path,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Protocols is the configuration for the supported protocols.
type Protocols struct {
	GRPC configoptional.Optional[configgrpc.ServerConfig] `mapstructure:"grpc"`
	HTTP configoptional.Optional[HTTPConfig]              `mapstructure:"http"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines configuration for OTLPGMI receiver.
type Config struct {
	// Protocols is the configuration for the supported protocols, currently gRPC and HTTP (Proto and JSON).
	Protocols `mapstructure:"protocols"`
	
	// License is the configuration for license validation
	License configoptional.Optional[LicenseConfig] `mapstructure:"license"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if !cfg.GRPC.HasValue() && !cfg.HTTP.HasValue() {
		return errors.New("must specify at least one protocol when using the OTLPGMI receiver")
	}
	
	// Validate license configuration if provided
	if cfg.License.HasValue() {
		license := cfg.License.Get()
		if err := license.Validate(); err != nil {
			return fmt.Errorf("license configuration is invalid: %w", err)
		}
	}
	
	return nil
}

// Validate checks the license configuration is valid
func (cfg *LicenseConfig) Validate() error {
	if cfg.ValidationURL == "" {
		return errors.New("validation_url is required")
	}
	
	// Validate URL format
	if _, err := url.Parse(cfg.ValidationURL); err != nil {
		return fmt.Errorf("validation_url is invalid: %w", err)
	}
	
	// Validate cache timeout
	if cfg.CacheTimeout < 0 {
		return errors.New("cache_timeout must be non-negative")
	}
	
	return nil
}
