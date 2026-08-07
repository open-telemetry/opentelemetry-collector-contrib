// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bearertokenauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
)

// Config specifies how the Per-RPC bearer token based authentication data should be obtained.
type Config struct {
	// Header specifies the auth-header for the token. Defaults to "Authorization"
	Header string `mapstructure:"header,omitempty"`

	// Scheme specifies the auth-scheme for the token. Defaults to "Bearer"
	Scheme string `mapstructure:"scheme,omitempty"`

	// BearerToken specifies the bearer token to use for every RPC.
	BearerToken configopaque.String `mapstructure:"token,omitempty"`

	// Tokens specifies multiple bearer tokens to use for every RPC.
	Tokens []configopaque.String `mapstructure:"tokens,omitempty"`

	// Filename points to a file that contains the bearer token(s) to use for every RPC.
	Filename string `mapstructure:"filename,omitempty"`

	// RetryOnFailure configures startup retry behavior when the file referenced by
	// Filename is not yet available. Disabled by default.
	RetryOnFailure RetryOnFailureConfig `mapstructure:"retry_on_failure,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// RetryOnFailureConfig configures retry-on-missing-file behavior for the file watcher.
type RetryOnFailureConfig struct {
	// Enabled, when true, makes startup retry reading the file referenced by
	// Filename instead of failing immediately when it is missing. Defaults to false.
	Enabled bool `mapstructure:"enabled,omitempty"`

	// InitialInterval is the time to wait before the first retry.
	InitialInterval time.Duration `mapstructure:"initial_interval,omitempty"`

	// MaxRetries is the maximum number of times to retry reading the file.
	MaxRetries int `mapstructure:"max_retries,omitempty"`

	// Offset is the interval between retries after the first.
	Offset time.Duration `mapstructure:"offset,omitempty"`
}

var (
	_                                  component.Config = (*Config)(nil)
	errNoTokenProvided                                  = errors.New("no bearer token provided")
	errTokensAndTokenProvided                           = errors.New("either tokens or token should be provided, not both")
	errRetryOnFailureNoFile                             = errors.New("retry_on_failure.enabled requires filename to be set")
	errRetryOnFailureInvalidMaxRetries                  = errors.New("retry_on_failure.max_retries must be greater than 0 when retry_on_failure.enabled is true")
	errRetryOnFailureInvalidOffset                      = errors.New("retry_on_failure.offset must be greater than 0 when retry_on_failure.enabled is true")
)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.BearerToken == "" && len(cfg.Tokens) == 0 && cfg.Filename == "" {
		return errNoTokenProvided
	}
	if cfg.BearerToken != "" && len(cfg.Tokens) > 0 {
		return errTokensAndTokenProvided
	}
	if cfg.RetryOnFailure.Enabled {
		if cfg.Filename == "" {
			return errRetryOnFailureNoFile
		}
		if cfg.RetryOnFailure.MaxRetries <= 0 {
			return errRetryOnFailureInvalidMaxRetries
		}
		if cfg.RetryOnFailure.Offset <= 0 {
			return errRetryOnFailureInvalidOffset
		}
	}
	return nil
}
