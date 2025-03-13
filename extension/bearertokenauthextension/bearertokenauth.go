// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bearertokenauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
)

var _ credentials.PerRPCCredentials = (*PerRPCAuth)(nil)

// PerRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type PerRPCAuth struct {
	auth *BearerTokenAuth
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (c *PerRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": c.auth.authorizationValue()}, nil
}

// RequireTransportSecurity always returns true for this implementation. Passing bearer tokens in plain-text connections is a bad idea.
func (c *PerRPCAuth) RequireTransportSecurity() bool {
	return true
}

var (
	_ extension.Extension      = (*BearerTokenAuth)(nil)
	_ extensionauth.Server     = (*BearerTokenAuth)(nil)
	_ extensionauth.HTTPClient = (*BearerTokenAuth)(nil)
	_ extensionauth.GRPCClient = (*BearerTokenAuth)(nil)
)

// BearerTokenAuth is an implementation of extensionauth interfaces. It embeds a static authorization "bearer" token in every rpc call.
type BearerTokenAuth struct {
	scheme                    string
	authorizationValuesAtomic atomic.Value

	shutdownCH chan struct{}

	filename string
	logger   *zap.Logger
}

func newBearerTokenAuth(cfg *Config, logger *zap.Logger) *BearerTokenAuth {
	if cfg.Filename != "" && (cfg.BearerToken != "" || len(cfg.Tokens) > 0) {
		logger.Warn("a filename is specified. Configured token(s) is ignored!")
	}
	a := &BearerTokenAuth{
		scheme:   cfg.Scheme,
		filename: cfg.Filename,
		logger:   logger,
	}
	switch {
	case len(cfg.Tokens) > 0:
		tokens := make([]string, len(cfg.Tokens))
		for i, token := range cfg.Tokens {
			tokens[i] = string(token)
		}
		a.setAuthorizationValues(tokens) // Store tokens
	case cfg.BearerToken != "":
		a.setAuthorizationValues([]string{string(cfg.BearerToken)}) // Store token
	case cfg.Filename != "":
		a.refreshToken() // Load tokens from file
	}
	return a
}

// Start of BearerTokenAuth does nothing and returns nil if no filename
// is specified. Otherwise a routine is started to monitor the file containing
// the token to be transferred.
func (b *BearerTokenAuth) Start(ctx context.Context, _ component.Host) error {
	if b.filename == "" {
		return nil
	}

	if b.shutdownCH != nil {
		return fmt.Errorf("bearerToken file monitoring is already running")
	}

	// Read file once
	b.refreshToken()

	b.shutdownCH = make(chan struct{})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	// start file watcher
	go b.startWatcher(ctx, watcher)

	return watcher.Add(b.filename)
}

func (b *BearerTokenAuth) startWatcher(ctx context.Context, watcher *fsnotify.Watcher) {
	defer watcher.Close()
	for {
		select {
		case _, ok := <-b.shutdownCH:
			_ = ok
			return
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				continue
			}
			// NOTE: k8s configmaps uses symlinks, we need this workaround.
			// original configmap file is removed.
			// SEE: https://martensson.io/go-fsnotify-and-kubernetes-configmaps/
			if event.Op == fsnotify.Remove || event.Op == fsnotify.Chmod {
				// remove the watcher since the file is removed
				if err := watcher.Remove(event.Name); err != nil {
					b.logger.Error(err.Error())
				}
				// add a new watcher pointing to the new symlink/file
				if err := watcher.Add(b.filename); err != nil {
					b.logger.Error(err.Error())
				}
				b.refreshToken()
			}
			// also allow normal files to be modified and reloaded.
			if event.Op == fsnotify.Write {
				b.refreshToken()
			}
		}
	}
}

// Reloads token from file
func (b *BearerTokenAuth) refreshToken() {
	b.logger.Info("refresh token", zap.String("filename", b.filename))
	tokenData, err := os.ReadFile(b.filename)
	if err != nil {
		b.logger.Error(err.Error())
		return
	}

	tokens := strings.Split(string(tokenData), "\n")
	for i, token := range tokens {
		tokens[i] = strings.TrimSpace(token)
	}
	b.setAuthorizationValues(tokens) // Stores new tokens
}

func (b *BearerTokenAuth) setAuthorizationValues(tokens []string) {
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
func (b *BearerTokenAuth) authorizationValues() []string {
	return b.authorizationValuesAtomic.Load().([]string)
}

// authorizationValue returns the first Authorization header/metadata value
// to set for client auth, and expected value for server auth.
func (b *BearerTokenAuth) authorizationValue() string {
	values := b.authorizationValues()
	if len(values) > 0 {
		return values[0] // Return the first token
	}
	return ""
}

// Shutdown of BearerTokenAuth does nothing and returns nil
func (b *BearerTokenAuth) Shutdown(_ context.Context) error {
	if b.filename == "" {
		return nil
	}

	if b.shutdownCH == nil {
		return fmt.Errorf("bearerToken file monitoring is not running")
	}
	b.shutdownCH <- struct{}{}
	close(b.shutdownCH)
	b.shutdownCH = nil
	return nil
}

// PerRPCCredentials returns PerRPCAuth an implementation of credentials.PerRPCCredentials that
func (b *BearerTokenAuth) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &PerRPCAuth{
		auth: b,
	}, nil
}

// RoundTripper is not implemented by BearerTokenAuth
func (b *BearerTokenAuth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &BearerAuthRoundTripper{
		baseTransport: base,
		auth:          b,
	}, nil
}

// Authenticate checks whether the given context contains valid auth data. Validates tokens from clients trying to access the service (incoming requests)
func (b *BearerTokenAuth) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	auth, ok := headers["authorization"]
	if !ok {
		auth, ok = headers["Authorization"]
	}
	if !ok || len(auth) == 0 {
		return ctx, errors.New("missing or empty authorization header")
	}
	token := auth[0] // Extract token from authorization header
	expectedTokens := b.authorizationValues()
	for _, expectedToken := range expectedTokens {
		if subtle.ConstantTimeCompare([]byte(expectedToken), []byte(token)) == 1 {
			return ctx, nil // Authentication successful, token is valid
		}
	}
	return ctx, fmt.Errorf("scheme or token does not match: %s", token) // Token is invalid
}

// BearerAuthRoundTripper intercepts and adds Bearer token Authorization headers to each http request.
type BearerAuthRoundTripper struct {
	baseTransport http.RoundTripper
	auth          *BearerTokenAuth
}

// RoundTrip modifies the original request and adds Bearer token Authorization headers. Incoming requests support multiple tokens, but outgoing requests only use one.
func (interceptor *BearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	req2.Header.Set("Authorization", interceptor.auth.authorizationValue())
	return interceptor.baseTransport.RoundTrip(req2)
}
