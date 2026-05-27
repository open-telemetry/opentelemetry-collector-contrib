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

	// FileRetry configures startup retry behavior when the file referenced by
	// Filename is not yet available. Disabled by default.
	FileRetry FileRetryConfig `mapstructure:"file_retry,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// FileRetryConfig configures retry-on-missing-file behavior for the file watcher.
type FileRetryConfig struct {
	// Enabled, when true, makes startup retry reading the file referenced by
	// Filename instead of failing immediately when it is missing. Defaults to false.
	Enabled bool `mapstructure:"enabled,omitempty"`

	// MaxRetries is the maximum number of times to retry reading the file when
	// Enabled is true.
	MaxRetries int `mapstructure:"max_retries,omitempty"`

	// RetryInterval is the interval between retries when Enabled is true.
	RetryInterval time.Duration `mapstructure:"retry_interval,omitempty"`
}

var (
	_                             component.Config = (*Config)(nil)
	errNoTokenProvided                             = errors.New("no bearer token provided")
	errTokensAndTokenProvided                      = errors.New("either tokens or token should be provided, not both")
	errFileRetryNoFile                             = errors.New("file_retry.enabled requires filename to be set")
	errFileRetryInvalidMaxRetries                  = errors.New("file_retry.max_retries must be greater than 0 when file_retry.enabled is true")
	errFileRetryInvalidInterval                    = errors.New("file_retry.retry_interval must be greater than 0 when file_retry.enabled is true")
)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.BearerToken == "" && len(cfg.Tokens) == 0 && cfg.Filename == "" {
		return errNoTokenProvided
	}
	if cfg.BearerToken != "" && len(cfg.Tokens) > 0 {
		return errTokensAndTokenProvided
	}
	if cfg.FileRetry.Enabled {
		if cfg.Filename == "" {
			return errFileRetryNoFile
		}
		if cfg.FileRetry.MaxRetries <= 0 {
			return errFileRetryInvalidMaxRetries
		}
		if cfg.FileRetry.RetryInterval <= 0 {
			return errFileRetryInvalidInterval
		}
	}
	return nil
}
