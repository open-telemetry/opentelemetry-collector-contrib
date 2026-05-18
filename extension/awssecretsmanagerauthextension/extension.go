// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	creds "google.golang.org/grpc/credentials"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension/internal/awsprovider"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth"
)

// Client extension

var (
	_ extension.Extension      = (*awsSecretsManagerAuthClient)(nil)
	_ extensionauth.HTTPClient = (*awsSecretsManagerAuthClient)(nil)
	_ extensionauth.GRPCClient = (*awsSecretsManagerAuthClient)(nil)
)

type clientCredentials struct {
	username string
	password string
}

type awsSecretsManagerAuthClient struct {
	provider *awsprovider.Provider
	creds    atomic.Pointer[clientCredentials]
	logger   *zap.Logger
}

func newClientAuthExtension(cfg *ClientAuthSettings, logger *zap.Logger) *awsSecretsManagerAuthClient {
	ext := &awsSecretsManagerAuthClient{logger: logger}
	ext.creds.Store(&clientCredentials{})

	ext.provider = awsprovider.NewProvider(&awsprovider.Config{
		SecretARN:       cfg.SecretARN,
		Region:          cfg.Region,
		RefreshInterval: cfg.RefreshInterval,
		Logger:          logger,
		FetchFunc: func(_ context.Context, secretValue string) error {
			var parsed map[string]string
			if err := json.Unmarshal([]byte(secretValue), &parsed); err != nil {
				return fmt.Errorf("parse secret JSON: %w", err)
			}
			username, ok := parsed[cfg.UsernameKey]
			if !ok {
				return fmt.Errorf("key %q not found in secret", cfg.UsernameKey)
			}
			password, ok := parsed[cfg.PasswordKey]
			if !ok {
				return fmt.Errorf("key %q not found in secret", cfg.PasswordKey)
			}
			ext.creds.Store(&clientCredentials{username: username, password: password})
			return nil
		},
	})

	return ext
}

func (c *awsSecretsManagerAuthClient) Start(ctx context.Context, host component.Host) error {
	return c.provider.Start(ctx, host)
}

func (c *awsSecretsManagerAuthClient) Shutdown(ctx context.Context) error {
	return c.provider.Shutdown(ctx)
}

func (c *awsSecretsManagerAuthClient) Username() string {
	return c.creds.Load().username
}

func (c *awsSecretsManagerAuthClient) Password() string {
	return c.creds.Load().password
}

func (c *awsSecretsManagerAuthClient) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return basicauth.NewRoundTripper(base, c)
}

func (c *awsSecretsManagerAuthClient) PerRPCCredentials() (creds.PerRPCCredentials, error) {
	return basicauth.NewPerRPCCredentials(c)
}

// Server extension

var (
	_ extension.Extension  = (*awsSecretsManagerAuthServer)(nil)
	_ extensionauth.Server = (*awsSecretsManagerAuthServer)(nil)
)

type awsSecretsManagerAuthServer struct {
	provider  *awsprovider.Provider
	matchFunc atomic.Pointer[func(string, string) bool]
	logger    *zap.Logger
}

func newServerAuthExtension(cfg *HtpasswdSettings, logger *zap.Logger) *awsSecretsManagerAuthServer {
	ext := &awsSecretsManagerAuthServer{logger: logger}

	ext.provider = awsprovider.NewProvider(&awsprovider.Config{
		SecretARN:       cfg.SecretARN,
		Region:          cfg.Region,
		RefreshInterval: cfg.RefreshInterval,
		Logger:          logger,
		FetchFunc: func(_ context.Context, secretValue string) error {
			content := secretValue
			if cfg.ValueKey != "" {
				var parsed map[string]string
				if err := json.Unmarshal([]byte(secretValue), &parsed); err != nil {
					return fmt.Errorf("parse secret JSON: %w", err)
				}
				v, ok := parsed[cfg.ValueKey]
				if !ok {
					return fmt.Errorf("key %q not found in secret", cfg.ValueKey)
				}
				content = v
			}
			htp, err := htpasswd.NewFromReader(strings.NewReader(content), htpasswd.DefaultSystems, nil)
			if err != nil {
				return fmt.Errorf("parse htpasswd content: %w", err)
			}
			matchFn := htp.Match
			ext.matchFunc.Store(&matchFn)
			return nil
		},
	})

	return ext
}

func (s *awsSecretsManagerAuthServer) Start(ctx context.Context, host component.Host) error {
	return s.provider.Start(ctx, host)
}

func (s *awsSecretsManagerAuthServer) Shutdown(ctx context.Context) error {
	return s.provider.Shutdown(ctx)
}

func (s *awsSecretsManagerAuthServer) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	fn := s.matchFunc.Load()
	if fn == nil {
		return ctx, fmt.Errorf("authenticator not started")
	}
	return basicauth.Authenticate(ctx, headers, *fn)
}
