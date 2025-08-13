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
	errNoClientIDProvided        = errors.New("no ClientID provided in the OAuth2 exporter configuration")
	errNoTokenURLProvided        = errors.New("no TokenURL provided in OAuth Client Credentials configuration")
	errNoClientSecretProvided    = errors.New("no ClientSecret provided in OAuth Client Credentials configuration")
	errNoSecretTokenTypeProvided = errors.New("no SecretTokenType provided in OAuth configuration")
	errNoSubjectTokenProvided    = errors.New("no SubjectToken or SubjectTokenFiel provided for STS mode")
	errUnsupportedMode           = errors.New("recieved an invalide auth_mode. Must be one of: client-credentials or sts")
)

// Config stores the configuration for OAuth2 Client Credentials (2-legged OAuth2 flow) setup.
type Config struct {
	//
	// ************ Client Credentials Authentication specific fields *********************************

	// ClientID is the application's ID.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-2.2
	ClientID string `mapstructure:"client_id"`

	// ClientIDFile is the file path to read the application's ID from.
	ClientIDFile string `mapstructure:"client_id_file"`

	// ClientSecret is the application's secret.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-2.3.1
	ClientSecret configopaque.String `mapstructure:"client_secret"`

	// ClientSecretFile is the file path to read the application's secret from.
	ClientSecretFile string `mapstructure:"client_secret_file"`

	//
	// ************ STS Token Exchange Authentication specific fields **************************

	// A security token that represents the identity of the party on behalf of whom the request is being made.
	SubjectToken string `mapstructure:"subject_token"`

	// SubjectTokenFile is the file path to read the SubjectToken from.
	SubjectTokenFile string `mapstructure:"subject_token_file"`

	// The type of security token provided.
	// See https://datatracker.ietf.org/doc/html/rfc8693#TokenTypeIdentifiers
	SubjectTokenType string `mapstructure:"subject_token_type"`

	// Name of the target service that we want to access
	Audience string `mapstructure:"audience"`

	// DisablegRPCTransportSecurity turns off TLS when using the credentials. i.e. sets TokenSource.RequireTransportSecurity to false. Only supported for STS mode.
	DisablegRPCTransportSecurity bool `mapstructure:"disable_grpc_transport_security, omitempty"`

	//
	// ******************************** Shared fields **********************************************

	// AuthMode specifies whether to use client credentials or STS flow.
	// Defaults to "client-credentials" unless "sts" is specified
	AuthMode string `mapstructure:"authmode, omitempty"`

	// TokenURL is the resource server's token endpoint
	// URL. This is a constant specific to each server.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-3.2
	TokenURL string `mapstructure:"token_url"`

	// EndpointParams specifies additional parameters for requests to the token endpoint.
	EndpointParams url.Values `mapstructure:"endpoint_params"`

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
	if cfg.TokenURL == "" {
		return errNoTokenURLProvided
	}
	switch cfg.AuthMode {
	case "", "client-credentials":
		// 2 Legged Oauth Mode
		if cfg.ClientID == "" && cfg.ClientIDFile == "" {
			return errNoClientIDProvided
		}
		if cfg.ClientSecret == "" && cfg.ClientSecretFile == "" {
			return errNoClientSecretProvided
		}
		return nil
	case "sts":
		// STS Token Exchange Mode
		if cfg.SubjectToken == "" && cfg.SubjectTokenFile == "" {
			return errNoSubjectTokenProvided
		}
		if cfg.SubjectTokenType == "" {
			return errNoSecretTokenTypeProvided
		}
		return nil
	}
	return errUnsupportedMode

}
