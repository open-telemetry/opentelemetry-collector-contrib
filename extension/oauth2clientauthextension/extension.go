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
	"google.golang.org/grpc/credentials"
	grpcOAuth "google.golang.org/grpc/credentials/oauth"
)

var (
	_ extension.Extension      = (*clientAuthenticator)(nil)
	_ extensionauth.HTTPClient = (*clientAuthenticator)(nil)
	_ extensionauth.GRPCClient = (*clientAuthenticator)(nil)
)

type TokenSourceConfiguration interface {
	TokenSource(context.Context) oauth2.TokenSource
	TokenEndpoint() string
}

// clientAuthenticator provides implementation for providing client authentication using OAuth2 client credentials
// workflow for both gRPC and HTTP clients.
type clientAuthenticator struct {
	component.StartFunc
	component.ShutdownFunc

	credentials TokenSourceConfiguration
	logger      *zap.Logger
	client      *http.Client
}

type errorWrappingTokenSource struct {
	ts       oauth2.TokenSource
	tokenURL string
}

// errorWrappingTokenSource implements TokenSource
var _ oauth2.TokenSource = (*errorWrappingTokenSource)(nil)

// errFailedToGetSecurityToken indicates a problem communicating with OAuth2 server.
var errFailedToGetSecurityToken = errors.New("failed to get security token from token endpoint")

func newClientAuthenticator(cfg *Config, logger *zap.Logger) (*clientAuthenticator, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	tlsCfg, err := cfg.TLS.LoadTLSConfig(context.Background())
	if err != nil {
		return nil, err
	}
	transport.TLSClientConfig = tlsCfg

	var credentials TokenSourceConfiguration

	switch cfg.GrantType {
	case grantTypeJWTBearer:
		credentials, err = newJwtGrantTypeConfig(cfg)
		if err != nil {
			return nil, err
		}
	case grantTypeClientCredentials, "":
		credentials, err = newJwtGrantTypeConfig(cfg)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown grant type %q", cfg.GrantType)
	}

	return &clientAuthenticator{
		credentials: credentials,
		logger:      logger,
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
func (o *clientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, o.client)
	return &oauth2.Transport{
		Source: errorWrappingTokenSource{
			ts:       o.credentials.TokenSource(ctx),
			tokenURL: o.credentials.TokenEndpoint(),
		},
		Base: base,
	}, nil
}

// PerRPCCredentials returns gRPC PerRPCCredentials that supports "client-credential" OAuth flow. The underneath
// oauth2.clientcredentials.Config instance will manage tokens performing auto refresh as necessary.
func (o *clientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, o.client)
	return grpcOAuth.TokenSource{
		TokenSource: errorWrappingTokenSource{
			ts:       o.credentials.TokenSource(ctx),
			tokenURL: o.credentials.TokenEndpoint(),
		},
	}, nil
}
