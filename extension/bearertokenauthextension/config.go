// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bearertokenauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"
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
	RetryOnFailure credentialsfile.RetryOnFailureConfig `mapstructure:"retry_on_failure,omitempty"`

	// WaitForTokenFile makes Start block until the token file is read successfully
	// (respecting RetryOnFailure) instead of retrying in the background. If the
	// file cannot be read within the retry budget, Start returns an error and
	// collector startup fails. Disabled by default.
	WaitForTokenFile bool `mapstructure:"wait_for_token_file,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var (
	_                                component.Config = (*Config)(nil)
	errNoTokenProvided                                = errors.New("no bearer token provided")
	errTokensAndTokenProvided                         = errors.New("either tokens or token should be provided, not both")
	errRetryOnFailureNoFile                           = errors.New("requires filename to be set")
	errWaitForTokenFileRequiresRetry                  = errors.New("wait_for_token_file requires retry_on_failure to be enabled")
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
		if err := cfg.RetryOnFailure.Validate(); err != nil {
			return err
		}
	}
	if cfg.WaitForTokenFile {
		if cfg.Filename == "" {
			return errRetryOnFailureNoFile
		}
		if !cfg.RetryOnFailure.Enabled {
			return errWaitForTokenFileRequiresRetry
		}
	}
	return nil
}
