// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bearertokenauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"

import (
	"errors"

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

	// prevent unkeyed literal initialization
	_ struct{}
}

var (
	_                         component.Config = (*Config)(nil)
	errNoTokenProvided                         = errors.New("no bearer token provided")
	errTokensAndTokenProvided                  = errors.New("either tokens or token should be provided, not both")
)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.BearerToken == "" && len(cfg.Tokens) == 0 && cfg.Filename == "" {
		return errNoTokenProvided
	}
	if cfg.BearerToken != "" && len(cfg.Tokens) > 0 {
		return errTokensAndTokenProvided
	}
	return nil
}
