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
)

var (
	_ extension.Extension      = (*clientAuthenticator)(nil)
	_ extensionauth.HTTPClient = (*clientAuthenticator)(nil)
	_ extensionauth.GRPCClient = (*clientAuthenticator)(nil)
	_ ContextTokenSource       = (*clientAuthenticator)(nil)
)

// errFailedToGetSecurityToken indicates a problem communicating with OAuth2 server.
var errFailedToGetSecurityToken = errors.New("failed to get security token from token endpoint")

type TokenSourceConfiguration interface {
	TokenSource(context.Context) oauth2.TokenSource
	TokenEndpoint() string
}

// ContextTokenSource provides an interface for obtaining an *oauth2.Token,
// with the given context.
type ContextTokenSource interface {
	Token(context.Context) (*oauth2.Token, error)
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
		credentials = newClientCredentialsGrantTypeConfig(cfg)
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

// RoundTripper wraps the provided base http.RoundTripper with an oauth2.Transport
// that injects OAuth2 tokens into outgoing HTTP requests. The returned RoundTripper
// will refresh tokens as needed using the context of the outgoing HTTP request.
func (o *clientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &roundTripper{o: o, base: base}, nil
}

type roundTripper struct {
	o    *clientAuthenticator
	base http.RoundTripper
}

func (rt *roundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	t := &oauth2.Transport{
		Source: rt.o.contextTokenSource(r.Context()),
		Base:   rt.base,
	}
	return t.RoundTrip(r)
}

// PerRPCCredentials returns a gRPC PerRPCCredentials that injects OAuth2 tokens into
// outgoing gRPC requests. The returned PerRPCCredentials will refresh tokens as needed
// using the request context.
func (o *clientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &perRPCCredentials{o: o}, nil
}

// Token returns an oauth2.Token, refreshing as needed with the provided context.
// The returned Token must not be modified.
func (o *clientAuthenticator) Token(ctx context.Context) (*oauth2.Token, error) {
	source := o.contextTokenSource(ctx)
	return source.Token()
}

// contextTokenSource returns an oauth2.TokenSource that can be used to obtain tokens
// with the provided context.
func (o *clientAuthenticator) contextTokenSource(ctx context.Context) errorWrappingTokenSource {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, o.client)
	return errorWrappingTokenSource{
		ts:       o.credentials.TokenSource(ctx),
		tokenURL: o.credentials.TokenEndpoint(),
	}
}

type errorWrappingTokenSource struct {
	ts       oauth2.TokenSource
	tokenURL string
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

// perRPCCredentials is based on google.golang.org/grpc/credentials/oauth.TokenSource,
// but passes through the request context.
type perRPCCredentials struct {
	o *clientAuthenticator
}

// GetRequestMetadata gets the request metadata as a map from a TokenSource.
func (c *perRPCCredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	token, err := c.o.Token(ctx)
	if err != nil {
		return nil, err
	}
	ri, _ := credentials.RequestInfoFromContext(ctx)
	if err = credentials.CheckSecurityLevel(ri.AuthInfo, credentials.PrivacyAndIntegrity); err != nil {
		return nil, fmt.Errorf("unable to transfer PerRPCCredentials: %w", err)
	}
	return map[string]string{
		"authorization": token.Type() + " " + token.AccessToken,
	}, nil
}

// RequireTransportSecurity indicates whether the credentials requires transport security.
func (*perRPCCredentials) RequireTransportSecurity() bool {
	return true
}
