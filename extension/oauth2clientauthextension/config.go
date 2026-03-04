// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"errors"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

var (
	errNoClientIDProvided          = errors.New("no ClientID provided in the OAuth2 exporter configuration")
	errNoClientCertificateProvided = errors.New("no ClientCertificateKey provided in the OAuth2 exporter configuration")
	errInvalidSignatureAlg         = errors.New("invalid signature algorithm")
	errNoTokenURLProvided          = errors.New("no TokenURL provided in OAuth Client Credentials configuration")
	errNoClientSecretProvided      = errors.New("no ClientSecret provided in OAuth Client Credentials configuration")
)

// Config stores the configuration for OAuth2 Client Credentials (2-legged OAuth2 flow) setup.
type Config struct {
	// ClientID is the application's ID.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-2.2
	ClientID string `mapstructure:"client_id"`

	// ClientIDFile is the file path to read the application's ID from.
	ClientIDFile string `mapstructure:"client_id_file"`

	// ClientSecret is the application's secret.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-2.3.1
	ClientSecret configopaque.String `mapstructure:"client_secret"`

	// ClientSecretFile is the file pathg to read the application's secret from.
	ClientSecretFile string `mapstructure:"client_secret_file"`

	// ClientCertificateKeyID is the Key ID to include in the jwt. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer".
	ClientCertificateKeyID string `mapstructure:"client_certificate_key_id"`

	// ClientCertificateKey is the application's private key. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer".
	ClientCertificateKey configopaque.String `mapstructure:"client_certificate_key"`

	// ClientSecrClientCertificateKeyFileetFile is the file pathg to read the application's secret from. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer".
	ClientCertificateKeyFile string `mapstructure:"client_certificate_key_file"`

	// GrantType is the OAuth2 grant type to use. It can be one of
	// "client_credentials" or "urn:ietf:params:oauth:grant-type:jwt-bearer" (RFC 7523).
	// Default value is "client_credentials"
	GrantType string `mapstructure:"grant_type"`

	// SignatureAlgorithm is the RSA algorithm used to sign JWT token. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer".
	// Default value is RS256 and valid values RS256, RS384, RS512
	SignatureAlgorithm string `mapstructure:"signature_algorithm,omitempty"`

	// Iss is the OAuth client identifier used when communicating with
	// the configured OAuth provider. Default value is client_id. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer".
	Iss string `mapstructure:"iss,omitempty"`

	// Audience optionally specifies the intended audience of the
	// request.  If empty, the value of TokenURL is used as the
	// intended audience. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer".
	Audience string `mapstructure:"audience,omitempty"`

	// Claims is a map of claims to be added to the JWT token. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer".
	Claims map[string]any `mapstructure:"claims,omitempty"`

	// EndpointParams specifies additional parameters for requests to the token endpoint.
	EndpointParams url.Values `mapstructure:"endpoint_params"`

	// TokenURL is the resource server's token endpoint
	// URL. This is a constant specific to each server.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-3.2
	TokenURL string `mapstructure:"token_url"`

	// Scope specifies optional requested permissions.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-3.3
	Scopes []string `mapstructure:"scopes,omitempty"`

	// TLS struct exposes TLS client configuration for the underneath client to authorization server.
	TLS configtls.ClientConfig `mapstructure:"tls,omitempty"`

	// Timeout parameter configures `http.Client.Timeout` for the underneath client to authorization
	// server while fetching and refreshing tokens.
	Timeout time.Duration `mapstructure:"timeout,omitempty"`

	// ExpiryBuffer specifies the time buffer before token expiry to refresh it.
	ExpiryBuffer time.Duration `mapstructure:"expiry_buffer,omitempty"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.ClientID == "" && cfg.ClientIDFile == "" {
		return errNoClientIDProvided
	}
	if cfg.GrantType == grantTypeJWTBearer {
		if cfg.ClientCertificateKey == "" && cfg.ClientCertificateKeyFile == "" {
			return errNoClientCertificateProvided
		}
	} else {
		if cfg.ClientSecret == "" && cfg.ClientSecretFile == "" {
			return errNoClientSecretProvided
		}
	}

	if cfg.TokenURL == "" {
		return errNoTokenURLProvided
	}
	return nil
}
