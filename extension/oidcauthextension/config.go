// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-jose/go-jose/v4"
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

	// Path to a local JWKS file containing public keys for token verification.
	// When provided, only keys from this file will be used - no remote key discovery will happen.
	// The file is watched for changes and keys are automatically reloaded on updates.
	// Optional.
	PublicKeysFile string `mapstructure:"public_keys_file"`
}

func (p *ProviderCfg) Validate() error {
	if p.Audience == "" && !p.IgnoreAudience {
		return errNoAudienceProvided
	}
	if p.IssuerURL == "" {
		return errNoIssuerURL
	}

	if p.PublicKeysFile != "" {
		if _, err := parseJWKSFile(p.PublicKeysFile); err != nil {
			return fmt.Errorf("invalid public_keys_file %q: %w", p.PublicKeysFile, err)
		}
	}

	return nil
}

var (
	supportedAlgorithms = map[string][]jose.SignatureAlgorithm{
		"rsa": {
			jose.RS256,
			jose.RS384,
			jose.RS512,
			jose.PS256,
			jose.PS384,
			jose.PS512,
		},
		"ecdsa": {
			jose.ES256,
			jose.ES384,
			jose.ES512,
		},
		"ed25519": {
			jose.EdDSA,
		},
	}

	allSupportedAlgorithms = slices.Concat(slices.Collect(maps.Values(supportedAlgorithms))...)
)

// staticPublicKey is a convenience struct used in the return value of
// parseJWKSFile. We cannot use ED25519 public keys as a map key (they
// are byte slices and thus unhashable), so this wrapper struct makes things
// a tiny bit more ergonomic.
type staticPublicKey struct {
	publicKey           crypto.PublicKey
	supportedAlgorithms []jose.SignatureAlgorithm
}

// parseJWKSFile reads and parses a JWKS file, returning a list of
// public keys and the signatures supported for each (based on the key's
// `alg` field if present, or the full list of algorithms possible for
// the key type if the field is not present). At the moment, only RSA,
// ECDSA, and ED25519 keys are supported, which is what go-oidc supports.
func parseJWKSFile(path string) ([]staticPublicKey, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for %q: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}

	var jwks jose.JSONWebKeySet
	if err := json.Unmarshal(data, &jwks); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS: %w", err)
	}

	publicKeys := []staticPublicKey{}
	for idx := range jwks.Keys {
		jwk := jwks.Keys[idx]

		alg := jose.SignatureAlgorithm(jwk.Algorithm)
		pk := staticPublicKey{
			publicKey: jwk.Public().Key,
		}

		var ktyp string
		switch pk.publicKey.(type) {
		case *rsa.PublicKey:
			ktyp = "rsa"
		case *ecdsa.PublicKey:
			ktyp = "ecdsa"
		case ed25519.PublicKey:
			ktyp = "ed25519"
		default:
			continue
		}

		if slices.Contains(supportedAlgorithms[ktyp], alg) {
			pk.supportedAlgorithms = append(pk.supportedAlgorithms, alg)
		} else {
			pk.supportedAlgorithms = supportedAlgorithms[ktyp]
		}

		publicKeys = append(publicKeys, pk)
	}

	if len(publicKeys) == 0 {
		return nil, errNoSupportedKeys
	}

	return publicKeys, nil
}
