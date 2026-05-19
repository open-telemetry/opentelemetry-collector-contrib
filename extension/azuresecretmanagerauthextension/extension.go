// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuresecretmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/azuresecretmanagerauthextension"

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/azuresecretmanagerauthextension/internal/azureprovider"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth"
)

// Client extension

var (
	_ extension.Extension      = (*azureSecretsManagerAuthClient)(nil)
	_ extensionauth.HTTPClient = (*azureSecretsManagerAuthClient)(nil)
	_ extensionauth.GRPCClient = (*azureSecretsManagerAuthClient)(nil)
)

type clientCredentials struct {
	username string
	password string
}

type azureSecretsManagerAuthClient struct {
	provider *azureprovider.Provider
	creds    atomic.Pointer[clientCredentials]
	logger   *zap.Logger
}

func newClientAuthExtension(cfg *ClientAuthSettings, logger *zap.Logger) *azureSecretsManagerAuthClient {
	ext := &azureSecretsManagerAuthClient{logger: logger}
	ext.creds.Store(&clientCredentials{})

	ext.provider = azureprovider.NewProvider(&azureprovider.Config{
		KeyVaultURI:     cfg.KeyVaultURI,
		SecretName:      cfg.SecretName,
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

func (c *azureSecretsManagerAuthClient) Start(ctx context.Context, host component.Host) error {
	return c.provider.Start(ctx, host)
}

func (c *azureSecretsManagerAuthClient) Shutdown(ctx context.Context) error {
	return c.provider.Shutdown(ctx)
}

func (c *azureSecretsManagerAuthClient) Username() string {
	return c.creds.Load().username
}

func (c *azureSecretsManagerAuthClient) Password() string {
	return c.creds.Load().password
}

func (c *azureSecretsManagerAuthClient) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return basicauth.NewRoundTripper(base, c)
}

func (c *azureSecretsManagerAuthClient) PerRPCCredentials() (creds.PerRPCCredentials, error) {
	return basicauth.NewPerRPCCredentials(c)
}

// Server extension

var (
	_ extension.Extension  = (*azureSecretsManagerAuthServer)(nil)
	_ extensionauth.Server = (*azureSecretsManagerAuthServer)(nil)
)

type azureSecretsManagerAuthServer struct {
	provider  *azureprovider.Provider
	matchFunc atomic.Pointer[func(string, string) bool]
	logger    *zap.Logger
}

func newServerAuthExtension(cfg *HtpasswdSettings, logger *zap.Logger) *azureSecretsManagerAuthServer {
	ext := &azureSecretsManagerAuthServer{logger: logger}

	ext.provider = azureprovider.NewProvider(&azureprovider.Config{
		KeyVaultURI:     cfg.KeyVaultURI,
		SecretName:      cfg.SecretName,
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

func (s *azureSecretsManagerAuthServer) Start(ctx context.Context, host component.Host) error {
	return s.provider.Start(ctx, host)
}

func (s *azureSecretsManagerAuthServer) Shutdown(ctx context.Context) error {
	return s.provider.Shutdown(ctx)
}

func (s *azureSecretsManagerAuthServer) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	fn := s.matchFunc.Load()
	if fn == nil {
		return ctx, errors.New("authenticator not started")
	}
	return basicauth.Authenticate(ctx, headers, *fn)
}
