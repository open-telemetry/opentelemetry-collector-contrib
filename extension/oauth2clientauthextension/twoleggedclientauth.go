// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"google.golang.org/grpc/credentials"
	grpcOAuth "google.golang.org/grpc/credentials/oauth"
)

// twoLeggedClientAuthenticator provides implementation for providing client authentication using OAuth2 client credentials
// workflow for both gRPC and HTTP clients.
type twoLeggedClientAuthenticator struct {
	clientCredentials *clientCredentialsConfig
	logger            *zap.Logger
	client            *http.Client
	component.StartFunc
	component.ShutdownFunc
}

var _ clientAuthenticator = (*twoLeggedClientAuthenticator)(nil)

func newTwoLeggedClientAuthenticator(cfg *Config, logger *zap.Logger) (*twoLeggedClientAuthenticator, error) {
	transport, err := createTransport(cfg)
	if err != nil {
		return nil, err
	}

	return &twoLeggedClientAuthenticator{
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

// roundTripper returns oauth2.Transport, an http.RoundTripper that performs "client-credential" OAuth flow and
// also auto refreshes OAuth tokens as needed.
func (o *twoLeggedClientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, o.client)
	return &oauth2.Transport{
		Source: errorWrappingTokenSource{
			ts:       o.clientCredentials.TokenSource(ctx),
			tokenURL: o.clientCredentials.TokenURL,
		},
		Base: base,
	}, nil
}

// perRPCCredentials returns gRPC PerRPCCredentials that supports "client-credential" OAuth flow. The underneath
// oauth2.clientcredentials.Config instance will manage tokens performing auto refresh as necessary.
func (o *twoLeggedClientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, o.client)
	return grpcOAuth.TokenSource{
		TokenSource: errorWrappingTokenSource{
			ts:       o.clientCredentials.TokenSource(ctx),
			tokenURL: o.clientCredentials.TokenURL,
		},
	}, nil
}
