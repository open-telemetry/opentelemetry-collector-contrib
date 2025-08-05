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
	errNoClientIDProvided     = errors.New("no ClientID provided in the OAuth2 exporter configuration")
	errNoTokenURLProvided     = errors.New("no TokenURL provided in OAuth Client Credentials configuration")
	errNoClientSecretProvided = errors.New("no ClientSecret provided in OAuth Client Credentials configuration")
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
	if cfg.ClientSecret == "" && cfg.ClientSecretFile == "" {
		return errNoClientSecretProvided
	}
	if cfg.TokenURL == "" {
		return errNoTokenURLProvided
	}
	return nil
}
