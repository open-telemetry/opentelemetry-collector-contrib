// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"

	"go.uber.org/multierr"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// clientCredentialsConfig is a clientcredentials.Config wrapper to allow
// values read from files in the ClientID and ClientSecret fields.
//
// Values from files can be retrieved by populating the ClientIDFile or
// the ClientSecretFile fields with the path to the file.
//
// Priority: File > Raw value
//
// Example - Retrieve secret from file:
//
//	cfg = newClientCredentialsConfig(&Config{
//		ClientID: "clientID",
//		ClientSecretFile: "/path/to/client/secret",
//		...,
//	})
type clientCredentialsConfig struct {
	clientcredentials.Config

	clientIDSource     valueSource
	clientSecretSource valueSource
}

type clientCredentialsTokenSource struct {
	ctx    context.Context
	config *clientCredentialsConfig
}

// clientCredentialsTokenSource implements TokenSource
var _ oauth2.TokenSource = (*clientCredentialsTokenSource)(nil)

// getClientID returns the actual client ID and abstracts the interface
func (c *clientCredentialsConfig) getClientID() (string, error) {
	return c.clientIDSource.getValue()
}

// getClientSecret returns the actual client secret and abstracts the interface
func (c *clientCredentialsConfig) getClientSecret() (string, error) {
	return c.clientSecretSource.getValue()
}

// createConfig creates a proper clientcredentials.Config with the actual config values
func (c *clientCredentialsConfig) createConfig() (*clientcredentials.Config, error) {
	clientID, err := c.getClientID()
	if err != nil {
		return nil, multierr.Combine(errNoClientIDProvided, err)
	}

	clientSecret, err := c.getClientSecret()
	if err != nil {
		return nil, multierr.Combine(errNoClientSecretProvided, err)
	}

	return &clientcredentials.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		TokenURL:       c.TokenURL,
		Scopes:         c.Scopes,
		EndpointParams: c.EndpointParams,
	}, nil
}

func (c *clientCredentialsConfig) TokenSource(ctx context.Context) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, clientCredentialsTokenSource{ctx: ctx, config: c})
}

func (ts clientCredentialsTokenSource) Token() (*oauth2.Token, error) {
	cfg, err := ts.config.createConfig()
	if err != nil {
		return nil, err
	}
	return cfg.TokenSource(ts.ctx).Token()
}

func newClientCredentialsConfig(cfg *Config) *clientCredentialsConfig {
	var clientIDSource valueSource
	clientIDSource = rawSource{cfg.ClientID}
	if len(cfg.ClientIDFile) > 0 {
		clientIDSource = fileSource{cfg.ClientIDFile}
	}

	var clientSecretSource valueSource
	clientSecretSource = rawSource{string(cfg.ClientSecret)}
	if len(cfg.ClientSecretFile) > 0 {
		clientSecretSource = fileSource{cfg.ClientSecretFile}
	}

	return &clientCredentialsConfig{
		Config: clientcredentials.Config{
			ClientID:       cfg.ClientID,
			ClientSecret:   string(cfg.ClientSecret),
			TokenURL:       cfg.TokenURL,
			Scopes:         cfg.Scopes,
			EndpointParams: cfg.EndpointParams,
		},

		clientIDSource:     clientIDSource,
		clientSecretSource: clientSecretSource,
	}
}
