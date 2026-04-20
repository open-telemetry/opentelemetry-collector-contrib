// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bearertokenauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"
)

var _ credentials.PerRPCCredentials = (*perRPCAuth)(nil)

// PerRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type perRPCAuth struct {
	auth *bearerTokenAuth
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (c *perRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{strings.ToLower(c.auth.header): c.auth.authorizationValue()}, nil
}

// RequireTransportSecurity always returns true for this implementation. Passing bearer tokens in plain-text connections is a bad idea.
func (*perRPCAuth) RequireTransportSecurity() bool {
	return true
}

var (
	_ extension.Extension      = (*bearerTokenAuth)(nil)
	_ extensionauth.Server     = (*bearerTokenAuth)(nil)
	_ extensionauth.HTTPClient = (*bearerTokenAuth)(nil)
	_ extensionauth.GRPCClient = (*bearerTokenAuth)(nil)
)

// BearerTokenAuth is an implementation of extensionauth interfaces. It embeds a static authorization "bearer" token in every rpc call.
type bearerTokenAuth struct {
	header                    string
	scheme                    string
	authorizationValuesAtomic atomic.Value

	tokenResolver credentialsfile.ValueResolver
	logger        *zap.Logger
}

func newBearerTokenAuth(cfg *Config, logger *zap.Logger) *bearerTokenAuth {
	a := &bearerTokenAuth{
		header: cfg.Header,
		scheme: cfg.Scheme,
		logger: logger,
	}

	var inlineToken string
	switch {
	case len(cfg.Tokens) > 0:
		tokens := make([]string, len(cfg.Tokens))
		for i, token := range cfg.Tokens {
			tokens[i] = string(token)
		}
		a.setAuthorizationValues(tokens)
		return a
	case cfg.BearerToken != "":
		inlineToken = string(cfg.BearerToken)
	}

	if cfg.Filename != "" && (cfg.BearerToken != "" || len(cfg.Tokens) > 0) {
		logger.Warn("a filename is specified. Configured token(s) is ignored!")
	}

	// Create token resolver for single token (inline or file)
	if cfg.Filename != "" || inlineToken != "" {
		resolver, err := credentialsfile.NewValueResolver(
			inlineToken,
			cfg.Filename,
			logger,
			credentialsfile.WithOnChange(func(_ string) {
				if cfg.Filename != "" {
					logger.Info("refresh token", zap.String("filename", cfg.Filename))
				}
				a.updateAuthorizationValues()
			}),
		)
		if err != nil {
			logger.Error("failed to create token resolver", zap.Error(err))
			return a
		}
		a.tokenResolver = resolver
		// Initialize token values
		a.updateAuthorizationValues()
	}

	return a
}

// Start of BearerTokenAuth does nothing and returns nil if no filename
// is specified. Otherwise a routine is started to monitor the file containing
// the token to be transferred.
func (b *bearerTokenAuth) Start(ctx context.Context, _ component.Host) error {
	if b.tokenResolver != nil {
		return b.tokenResolver.Start(ctx)
	}
	return nil
}

func (b *bearerTokenAuth) updateAuthorizationValues() {
	if b.tokenResolver == nil {
		return
	}
	tokenData := b.tokenResolver.Value()
	tokens := strings.Split(tokenData, "\n")
	for i, token := range tokens {
		tokens[i] = strings.TrimSpace(token)
	}
	b.setAuthorizationValues(tokens)
}

func (b *bearerTokenAuth) setAuthorizationValues(tokens []string) {
	values := make([]string, len(tokens))
	for i, token := range tokens {
		if b.scheme != "" {
			values[i] = b.scheme + " " + token
		} else {
			values[i] = token
		}
	}
	b.authorizationValuesAtomic.Store(values)
}

// authorizationValues returns the Authorization header/metadata values
// to set for client auth, and expected values for server auth.
func (b *bearerTokenAuth) authorizationValues() []string {
	return b.authorizationValuesAtomic.Load().([]string)
}

// authorizationValue returns the first Authorization header/metadata value
// to set for client auth, and expected value for server auth.
func (b *bearerTokenAuth) authorizationValue() string {
	values := b.authorizationValues()
	if len(values) > 0 {
		return values[0] // Return the first token
	}
	return ""
}

// Shutdown of BearerTokenAuth does nothing and returns nil
func (b *bearerTokenAuth) Shutdown(_ context.Context) error {
	if b.tokenResolver != nil {
		return b.tokenResolver.Shutdown()
	}
	return nil
}

// PerRPCCredentials returns PerRPCAuth an implementation of credentials.PerRPCCredentials that
func (b *bearerTokenAuth) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &perRPCAuth{
		auth: b,
	}, nil
}

// RoundTripper is not implemented by BearerTokenAuth
func (b *bearerTokenAuth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &bearerAuthRoundTripper{
		header:        b.header,
		baseTransport: base,
		auth:          b,
	}, nil
}

// Authenticate checks whether the given context contains valid auth data. Validates tokens from clients trying to access the service (incoming requests)
func (b *bearerTokenAuth) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	// Use canonical header key to match how Go's HTTP server stores headers
	auth, ok := headers[http.CanonicalHeaderKey(b.header)]

	// Also check lower-case header key to support gRPC metadata format
	if !ok {
		auth, ok = headers[strings.ToLower(b.header)]
	}

	if !ok || len(auth) == 0 {
		return ctx, fmt.Errorf("missing or empty authorization header: %s", b.header)
	}
	token := auth[0] // Extract token from authorization header
	expectedTokens := b.authorizationValues()
	for _, expectedToken := range expectedTokens {
		if subtle.ConstantTimeCompare([]byte(expectedToken), []byte(token)) == 1 {
			return ctx, nil // Authentication successful, token is valid
		}
	}
	return ctx, errors.New("provided authorization does not match expected scheme or token") // Token is invalid
}

// BearerAuthRoundTripper intercepts and adds Bearer token Authorization headers to each http request.
type bearerAuthRoundTripper struct {
	header        string
	baseTransport http.RoundTripper
	auth          *bearerTokenAuth
}

// RoundTrip modifies the original request and adds Bearer token Authorization headers. Incoming requests support multiple tokens, but outgoing requests only use one.
func (interceptor *bearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	req2.Header.Set(interceptor.header, interceptor.auth.authorizationValue())
	return interceptor.baseTransport.RoundTrip(req2)
}
