// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/multierr"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	grantTypeClientCredentials = "client_credentials"
)

func newClientCredentialsGrantTypeConfig(cfg *Config) *clientCredentialsConfig {
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
		ExpiryBuffer:     cfg.ExpiryBuffer,
	}
}

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
//	cfg := ClientCredentialsConfig{
//		Config: Config{
//			ClientID:     "clientId",
//			...
//		},
//		ClientSecretFile: "/path/to/client/secret",
//	}
type ClientCredentialsConfig struct {
	Config Config

	AuthStyle    oauth2.AuthStyle
	ExpiryBuffer int
}

// clientCredentialsConfig is an internal version that embeds clientcredentials.Config
type clientCredentialsConfig struct {
	clientcredentials.Config

	ClientIDFile     string
	ClientSecretFile string
	ExpiryBuffer     time.Duration
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
	if filepath != "" {
		return readCredentialsFile(filepath)
	}

	return value, nil
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

// TokenSource creates an oauth2.TokenSource from the exported ClientCredentialsConfig
func (c *ClientCredentialsConfig) TokenSource(ctx context.Context) oauth2.TokenSource {
	internalConfig := &clientCredentialsConfig{
		Config: clientcredentials.Config{
			ClientID:       c.Config.ClientID,
			ClientSecret:   string(c.Config.ClientSecret),
			TokenURL:       c.Config.TokenURL,
			Scopes:         c.Config.Scopes,
			EndpointParams: c.Config.EndpointParams,
			AuthStyle:      c.AuthStyle,
		},
		ClientIDFile:     c.Config.ClientIDFile,
		ClientSecretFile: c.Config.ClientSecretFile,
		ExpiryBuffer:     time.Duration(c.ExpiryBuffer) * time.Second,
	}
	return internalConfig.tokenSource(ctx)
}

func (c *clientCredentialsConfig) tokenSource(ctx context.Context) oauth2.TokenSource {
	return oauth2.ReuseTokenSourceWithExpiry(nil, clientCredentialsTokenSource{ctx: ctx, config: c}, c.ExpiryBuffer)
}

func (c *clientCredentialsConfig) TokenEndpoint() string {
	return c.TokenURL
}

func (ts clientCredentialsTokenSource) Token() (*oauth2.Token, error) {
	cfg, err := ts.config.createConfig()
	if err != nil {
		return nil, err
	}
	return cfg.TokenSource(ts.ctx).Token()
}
