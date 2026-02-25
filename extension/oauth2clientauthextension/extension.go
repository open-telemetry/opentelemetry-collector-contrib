// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
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

	credentials  TokenSourceConfiguration
	logger       *zap.Logger
	client       *http.Client
	expiryBuffer time.Duration

	// sem is a buffered channel of size 1 used as a context-aware mutex
	// to protect token access/refresh.
	sem   chan struct{}
	token *oauth2.Token
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
		credentials:  credentials,
		logger:       logger,
		expiryBuffer: cfg.ExpiryBuffer,
		sem:          make(chan struct{}, 1),
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
	token, err := rt.o.Token(r.Context())
	if err != nil {
		return nil, err
	}
	r2 := r.Clone(r.Context())
	token.SetAuthHeader(r2)
	return rt.base.RoundTrip(r2)
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
	select {
	case o.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-o.sem }()

	if o.token.Valid() && (o.token.Expiry.IsZero() || time.Until(o.token.Expiry) > o.expiryBuffer) {
		return o.token, nil
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, o.client)
	ts := o.credentials.TokenSource(ctx)
	tok, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf(
			"%w (endpoint %q): %w",
			errFailedToGetSecurityToken, o.credentials.TokenEndpoint(), err,
		)
	}
	o.token = tok
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
