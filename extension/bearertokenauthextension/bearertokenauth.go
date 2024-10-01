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
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
)

var _ credentials.PerRPCCredentials = (*PerRPCAuth)(nil)

// PerRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type PerRPCAuth struct {
	metadata map[string]string
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (c *PerRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return c.metadata, nil
}

// RequireTransportSecurity always returns true for this implementation. Passing bearer tokens in plain-text connections is a bad idea.
func (c *PerRPCAuth) RequireTransportSecurity() bool {
	return true
}

var (
	_ auth.Server = (*BearerTokenAuth)(nil)
	_ auth.Client = (*BearerTokenAuth)(nil)
)

// BearerTokenAuth is an implementation of auth.Client. It embeds a static authorization "bearer" token in every rpc call.
type BearerTokenAuth struct {
	scheme                   string
	authorizationValueAtomic atomic.Value

	shutdownCH chan struct{}

	filename string
	logger   *zap.Logger
}

var _ auth.Client = (*BearerTokenAuth)(nil)

func newBearerTokenAuth(cfg *Config, logger *zap.Logger) *BearerTokenAuth {
	if cfg.Filename != "" && cfg.BearerToken != "" {
		logger.Warn("a filename is specified. Configured token is ignored!")
	}
	a := &BearerTokenAuth{
		scheme:   cfg.Scheme,
		filename: cfg.Filename,
		logger:   logger,
	}
	a.setAuthorizationValue(string(cfg.BearerToken))
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

func (b *BearerTokenAuth) refreshToken() {
	b.logger.Info("refresh token", zap.String("filename", b.filename))
	token, err := os.ReadFile(b.filename)
	if err != nil {
		b.logger.Error(err.Error())
		return
	}
	b.setAuthorizationValue(string(token))
}

func (b *BearerTokenAuth) setAuthorizationValue(token string) {
	value := token
	if b.scheme != "" {
		value = b.scheme + " " + value
	}
	b.authorizationValueAtomic.Store(value)
}

// authorizationValue returns the Authorization header/metadata value
// to set for client auth, and expected value for server auth.
func (b *BearerTokenAuth) authorizationValue() string {
	return b.authorizationValueAtomic.Load().(string)
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
		metadata: map[string]string{"authorization": b.authorizationValue()},
	}, nil
}

// RoundTripper is not implemented by BearerTokenAuth
func (b *BearerTokenAuth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &BearerAuthRoundTripper{
		baseTransport: base,
		auth:          b,
	}, nil
}

// Authenticate checks whether the given context contains valid auth data.
func (b *BearerTokenAuth) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	auth, ok := headers["authorization"]
	if !ok {
		auth, ok = headers["Authorization"]
	}
	if !ok || len(auth) == 0 {
		return ctx, errors.New("missing or empty authorization header")
	}
	token := auth[0]
	expect := b.authorizationValue()
	if subtle.ConstantTimeCompare([]byte(expect), []byte(token)) == 0 {
		return ctx, fmt.Errorf("scheme or token does not match: %s", token)
	}
	return ctx, nil
}

// BearerAuthRoundTripper intercepts and adds Bearer token Authorization headers to each http request.
type BearerAuthRoundTripper struct {
	baseTransport http.RoundTripper
	auth          *BearerTokenAuth
}

// RoundTrip modifies the original request and adds Bearer token Authorization headers.
func (interceptor *BearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	req2.Header.Set("Authorization", interceptor.auth.authorizationValue())
	return interceptor.baseTransport.RoundTrip(req2)
}
