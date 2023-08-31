// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// clientCredentialsConfig is a clientcredentials.Config wrapper.
type clientCredentialsConfig struct {
	clientcredentials.Config
}

type clientCredentialsTokenSource struct {
	ctx    context.Context
	config *clientCredentialsConfig
}

// clientCredentialsTokenSource implements TokenSource
var _ oauth2.TokenSource = (*clientCredentialsTokenSource)(nil)

// createConfig creates a proper clientcredentials.Config with values retrieved
// from files, if the user has specified a "file:" prefix
func (c *clientCredentialsConfig) createConfig() (*clientcredentials.Config, error) {
	return &clientcredentials.Config{
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
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
