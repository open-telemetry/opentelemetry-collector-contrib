// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"errors"
	"fmt"
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
	errInvalidAuthMethod           = errors.New("invalid token_endpoint_auth_method")
	errClientSecretWithPrivateKey  = errors.New("client_secret and client_secret_file must not be set when token_endpoint_auth_method is private_key_jwt")
	errAuthMethodWithJWTBearer     = errors.New("token_endpoint_auth_method is not supported with the jwt-bearer grant type")
)

const (
	authMethodClientSecretBasic = "client_secret_basic"
	authMethodClientSecretPost  = "client_secret_post"
	authMethodPrivateKeyJWT     = "private_key_jwt"
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
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer" or
	// TokenEndpointAuthMethod is set to "private_key_jwt".
	ClientCertificateKeyID string `mapstructure:"client_certificate_key_id"`

	// ClientCertificateKey is the application's PEM encoded RSA private key. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer" or
	// TokenEndpointAuthMethod is set to "private_key_jwt".
	ClientCertificateKey configopaque.String `mapstructure:"client_certificate_key"`

	// ClientCertificateKeyFile is the file path to read the application's private key from. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer" or
	// TokenEndpointAuthMethod is set to "private_key_jwt".
	ClientCertificateKeyFile string `mapstructure:"client_certificate_key_file"`

	// GrantType is the OAuth2 grant type to use. It can be one of
	// "client_credentials" or "urn:ietf:params:oauth:grant-type:jwt-bearer" (RFC 7523).
	// Default value is "client_credentials"
	GrantType string `mapstructure:"grant_type"`

	// TokenEndpointAuthMethod is the method used to authenticate the client at the
	// token endpoint when GrantType is "client_credentials". It can be one of
	// "client_secret_basic", "client_secret_post" or "private_key_jwt" (RFC 7523 section 2.2).
	// If empty, the client secret is sent and the placement (header or body) is auto-detected.
	TokenEndpointAuthMethod string `mapstructure:"token_endpoint_auth_method,omitempty"`

	// SignatureAlgorithm is the RSA algorithm used to sign JWT token. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer" or
	// TokenEndpointAuthMethod is set to "private_key_jwt".
	// Default value is RS256 and valid values RS256, RS384, RS512
	SignatureAlgorithm string `mapstructure:"signature_algorithm,omitempty"`

	// Iss is the OAuth client identifier used when communicating with
	// the configured OAuth provider. Default value is client_id. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer".
	Iss string `mapstructure:"iss,omitempty"`

	// Audience optionally specifies the "aud" claim of the signed JWT.
	// If empty, the value of TokenURL is used. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer" or
	// TokenEndpointAuthMethod is set to "private_key_jwt".
	Audience string `mapstructure:"audience,omitempty"`

	// Claims is a map of claims to be added to the JWT token. Only used if
	// GrantType is set to "urn:ietf:params:oauth:grant-type:jwt-bearer" or
	// TokenEndpointAuthMethod is set to "private_key_jwt".
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
	switch cfg.TokenEndpointAuthMethod {
	case "", authMethodClientSecretBasic, authMethodClientSecretPost, authMethodPrivateKeyJWT:
	default:
		return fmt.Errorf("%w %q", errInvalidAuthMethod, cfg.TokenEndpointAuthMethod)
	}
	hasKey := cfg.ClientCertificateKey != "" || cfg.ClientCertificateKeyFile != ""
	hasSecret := cfg.ClientSecret != "" || cfg.ClientSecretFile != ""
	switch {
	case cfg.GrantType == grantTypeJWTBearer:
		if cfg.TokenEndpointAuthMethod != "" {
			return errAuthMethodWithJWTBearer
		}
		if !hasKey {
			return errNoClientCertificateProvided
		}
	case cfg.TokenEndpointAuthMethod == authMethodPrivateKeyJWT:
		if !hasKey {
			return errNoClientCertificateProvided
		}
		if hasSecret {
			return errClientSecretWithPrivateKey
		}
	default:
		if !hasSecret {
			return errNoClientSecretProvided
		}
	}

	if cfg.TokenURL == "" {
		return errNoTokenURLProvided
	}
	return nil
}
