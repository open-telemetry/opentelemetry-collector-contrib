// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"google.golang.org/grpc/credentials"
	grpcOAuth "google.golang.org/grpc/credentials/oauth"
)

var (
	_ extension.Extension      = (*OauthClientAuthenticator)(nil)
	_ extensionauth.HTTPClient = (*OauthClientAuthenticator)(nil)
	_ extensionauth.GRPCClient = (*OauthClientAuthenticator)(nil)
)

// OauthClientAuthenticator provides implementation for providing client authentication using OAuth2 client credentials
// workflow for both gRPC and HTTP clients.
type OauthClientAuthenticator struct {
	component.StartFunc
	component.ShutdownFunc

	clientCredentials *clientCredentialsConfig
	logger            *zap.Logger
	client            *http.Client
}

type errorWrappingTokenSource struct {
	ts       oauth2.TokenSource
	tokenURL string
}

// errorWrappingTokenSource implements TokenSource
var _ oauth2.TokenSource = (*errorWrappingTokenSource)(nil)

// errFailedToGetSecurityToken indicates a problem communicating with OAuth2 server.
var errFailedToGetSecurityToken = errors.New("failed to get security token from token endpoint")

func newClientAuthenticator(cfg *Config, logger *zap.Logger) (*OauthClientAuthenticator, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	tlsCfg, err := cfg.TLSSetting.LoadTLSConfig(context.Background())
	if err != nil {
		return nil, err
	}
	transport.TLSClientConfig = tlsCfg

	return &OauthClientAuthenticator{
		clientCredentials: &clientCredentialsConfig{
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
		},
		logger: logger,
		client: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
	}, nil
}

func (ewts errorWrappingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := ewts.ts.Token()
	if err != nil {
		return tok, multierr.Combine(
			fmt.Errorf("%w (endpoint %q)", errFailedToGetSecurityToken, ewts.tokenURL),
			err)
	}
	return tok, nil
}

// RoundTripper returns oauth2.Transport, an http.RoundTripper that performs "client-credential" OAuth flow and
// also auto refreshes OAuth tokens as needed.
func (o *OauthClientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, o.client)
	return &oauth2.Transport{
		Source: errorWrappingTokenSource{
			ts:       o.clientCredentials.TokenSource(ctx),
			tokenURL: o.clientCredentials.TokenURL,
		},
		Base: base,
	}, nil
}

// PerRPCCredentials returns gRPC PerRPCCredentials that supports "client-credential" OAuth flow. The underneath
// oauth2.clientcredentials.Config instance will manage tokens performing auto refresh as necessary.
func (o *OauthClientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, o.client)
	return grpcOAuth.TokenSource{
		TokenSource: errorWrappingTokenSource{
			ts:       o.clientCredentials.TokenSource(ctx),
			tokenURL: o.clientCredentials.TokenURL,
		},
	}, nil
}
