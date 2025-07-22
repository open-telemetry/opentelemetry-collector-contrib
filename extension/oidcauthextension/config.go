// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import (
	"fmt"

	"go.uber.org/multierr"
)

// Config has the configuration for the OIDC Authenticator extension.
type Config struct {
	// The attribute (header name) to look for auth data. Optional, default value: "authorization".
	Attribute string `mapstructure:"attribute"`

	// Deprecated: use Providers instead.
	// IssuerURL is the base URL for the OIDC provider.
	// Required.
	IssuerURL string `mapstructure:"issuer_url"`

	// Deprecated: use Providers instead.
	// Audience of the token, used during the verification.
	// For example: "https://accounts.google.com" or "https://login.salesforce.com".
	// Required unless IgnoreAudience is true.
	Audience string `mapstructure:"audience"`

	// Deprecated: use Providers instead.
	// When true, this skips validating the audience field.
	// Optional.
	IgnoreAudience bool `mapstructure:"ignore_audience"`

	// Deprecated: use Providers instead.
	// The local path for the issuer CA's TLS server cert.
	// Optional.
	IssuerCAPath string `mapstructure:"issuer_ca_path"`

	// Deprecated: use Providers instead.
	// The claim to use as the username, in case the token's 'sub' isn't the suitable source.
	// Optional.
	UsernameClaim string `mapstructure:"username_claim"`

	// Deprecated: use Providers instead.
	// The claim that holds the subject's group membership information.
	// Optional.
	GroupsClaim string `mapstructure:"groups_claim"`

	// Providers allows configuring multiple OIDC providers.
	// Use the getProviderConfigs() method to get the full list of providers, including the legacy configuration.
	Providers []ProviderCfg `mapstructure:"providers"`
}

func (cfg *Config) getLegacyProviderConfig() *ProviderCfg {
	if cfg.IssuerURL != "" ||
		cfg.Audience != "" ||
		cfg.IgnoreAudience ||
		cfg.IssuerCAPath != "" ||
		cfg.UsernameClaim != "" ||
		cfg.GroupsClaim != "" {
		return &ProviderCfg{
			IssuerURL:      cfg.IssuerURL,
			Audience:       cfg.Audience,
			IgnoreAudience: cfg.IgnoreAudience,
			IssuerCAPath:   cfg.IssuerCAPath,
			UsernameClaim:  cfg.UsernameClaim,
			GroupsClaim:    cfg.GroupsClaim,
		}
	}
	return nil
}

// getProviderConfigs returns a slice of ProviderCfg, including the legacy provider configuration
// if it exists. The legacy configuration is prepended to the list of providers.
// Prefer this function over accessing the Providers field directly.
func (cfg *Config) getProviderConfigs() []ProviderCfg {
	if legacyProvider := cfg.getLegacyProviderConfig(); legacyProvider != nil {
		return append([]ProviderCfg{*legacyProvider}, cfg.Providers...)
	}
	return cfg.Providers
}

func (cfg *Config) Validate() (errs error) {
	seenIssuers := make(map[string]struct{})
	for _, provider := range cfg.getProviderConfigs() {
		if _, exists := seenIssuers[provider.IssuerURL]; exists {
			errs = multierr.Append(errs, fmt.Errorf("duplicate issuer URL found: %s", provider.IssuerURL))
			continue
		}
		seenIssuers[provider.IssuerURL] = struct{}{}
		errs = multierr.Append(errs, provider.Validate())
	}
	return errs
}

type ProviderCfg struct {
	// IssuerURL is the base URL for the OIDC provider.
	// Required.
	IssuerURL string `mapstructure:"issuer_url"`

	// Audience of the token, used during the verification.
	// For example: "https://accounts.google.com" or "https://login.salesforce.com".
	// Required unless IgnoreAudience is true.
	Audience string `mapstructure:"audience"`

	// When true, this skips validating the audience field.
	// Optional.
	IgnoreAudience bool `mapstructure:"ignore_audience"`

	// The local path for the issuer CA's TLS server cert.
	// Optional.
	IssuerCAPath string `mapstructure:"issuer_ca_path"`

	// The claim to use as the username, in case the token's 'sub' isn't the suitable source.
	// Optional.
	UsernameClaim string `mapstructure:"username_claim"`

	// The claim that holds the subject's group membership information.
	// Optional.
	GroupsClaim string `mapstructure:"groups_claim"`
}

func (p *ProviderCfg) Validate() error {
	if p.Audience == "" && !p.IgnoreAudience {
		return errNoAudienceProvided
	}
	if p.IssuerURL == "" {
		return errNoIssuerURL
	}
	return nil
}
