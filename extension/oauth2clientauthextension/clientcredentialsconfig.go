// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"
	"fmt"
	"os"
	"strings"

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

	ClientIDFile     string
	ClientSecretFile string

	clientIDSource     valueSource
	clientSecretSource valueSource
}

type clientCredentialsTokenSource struct {
	ctx    context.Context
	config *clientCredentialsConfig
}

// clientCredentialsTokenSource implements TokenSource
var _ oauth2.TokenSource = (*clientCredentialsTokenSource)(nil)

func readCredentialsFile(path string) (string, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read credentials file %q: %w", path, err)
	}

	credential := strings.TrimSpace(string(f))
	if credential == "" {
		return "", fmt.Errorf("empty credentials file %q", path)
	}
	return credential, nil
}

func getActualValue(value, filepath string) (string, error) {
	if len(filepath) > 0 {
		return readCredentialsFile(filepath)
	}

	return value, nil
}

// getClientID returns the actual client ID and abstracts the interface
func (c *clientCredentialsConfig) getClientID() (string, error) {
	return c.clientIDSource.getValue()
}

// getClientSecret returns the actual client secret and abstracts the interface
func (c *clientCredentialsConfig) getClientSecret() (string, error) {
	return c.clientSecretSource.getValue()
}

// createConfig creates a proper clientcredentials.Config with values retrieved
// from files, if the user has specified '*_file' values
func (c *clientCredentialsConfig) createConfig() (*clientcredentials.Config, error) {
	clientID, err := getActualValue(c.ClientID, c.ClientIDFile)
	if err != nil {
		return nil, multierr.Combine(errNoClientIDProvided, err)
	}

	clientSecret, err := getActualValue(c.ClientSecret, c.ClientSecretFile)
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
		ClientIDFile:     cfg.ClientIDFile,
		ClientSecretFile: cfg.ClientSecretFile,

		clientIDSource:     clientIDSource,
		clientSecretSource: clientSecretSource,
	}
}
