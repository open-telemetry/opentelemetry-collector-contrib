// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"
	creds "google.golang.org/grpc/credentials"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/secretprovider"
)

var (
	errNoAuth              = basicauth.ErrNoAuth
	errInvalidCredentials  = basicauth.ErrInvalidCredentials
	errInvalidSchemePrefix = basicauth.ErrInvalidSchemePrefix
	errInvalidFormat       = basicauth.ErrInvalidFormat
)

func newClientAuthExtension(cfg *Config) *basicAuthClient {
	return &basicAuthClient{clientAuth: cfg.ClientAuth}
}

func newServerAuthExtension(cfg *Config) (*basicAuthServer, error) {
	if cfg.Htpasswd == nil || (cfg.Htpasswd.File == "" && cfg.Htpasswd.Inline == "" && cfg.Htpasswd.SecretProvider == nil) {
		return nil, errNoCredentialSource
	}

	return &basicAuthServer{
		htpasswd: cfg.Htpasswd,
	}, nil
}

var (
	_ extension.Extension            = (*basicAuthServer)(nil)
	_ extensionauth.Server           = (*basicAuthServer)(nil)
	_ extensioncapabilities.Dependent = (*basicAuthServer)(nil)
)

type htpasswdMatcher struct {
	htp *htpasswd.File
}

func (m *htpasswdMatcher) verify(username, password string) bool {
	return m.htp.Match(username, password)
}

type basicAuthServer struct {
	htpasswd *HtpasswdSettings
	matcher  atomic.Pointer[htpasswdMatcher]
	logger   *zap.Logger
}

func (ba *basicAuthServer) Start(ctx context.Context, host component.Host) error {
	if ba.htpasswd.SecretProvider != nil {
		cfg := ba.htpasswd.SecretProvider
		ext, ok := host.GetExtensions()[cfg.ID]
		if !ok {
			return fmt.Errorf("secret provider extension %q not found", cfg.ID)
		}
		sp, ok := ext.(secretprovider.SecretProvider)
		if !ok {
			return fmt.Errorf("extension %q does not implement SecretProvider", cfg.ID)
		}

		raw, err := sp.GetSecret(ctx)
		if err != nil {
			return fmt.Errorf("initial secret fetch from %q: %w", cfg.ID, err)
		}
		if err := ba.updateHtpasswd(raw); err != nil {
			return fmt.Errorf("parse htpasswd from secret provider: %w", err)
		}

		sp.OnChange(func(newValue string) {
			if err := ba.updateHtpasswd(newValue); err != nil {
				ba.logger.Error("failed to parse refreshed htpasswd content", zap.Error(err))
				return
			}
			ba.logger.Info("htpasswd updated from secret provider")
		})
		return nil
	}

	var rs []io.Reader

	if ba.htpasswd.File != "" {
		f, err := os.Open(ba.htpasswd.File)
		if err != nil {
			return fmt.Errorf("open htpasswd file: %w", err)
		}
		defer f.Close()

		rs = append(rs, f, strings.NewReader("\n"))
	}

	// Ensure that the inline content is read the last.
	// This way the inline content will override the content from file.
	rs = append(rs, strings.NewReader(ba.htpasswd.Inline))

	mr := io.MultiReader(rs...)
	htp, err := htpasswd.NewFromReader(mr, htpasswd.DefaultSystems, nil)
	if err != nil {
		return fmt.Errorf("read htpasswd content: %w", err)
	}

	ba.matcher.Store(&htpasswdMatcher{htp: htp})
	return nil
}

func (ba *basicAuthServer) Shutdown(_ context.Context) error {
	return nil
}

func (ba *basicAuthServer) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	m := ba.matcher.Load()
	if m == nil {
		return ctx, fmt.Errorf("htpasswd not yet initialized")
	}
	return basicauth.Authenticate(ctx, headers, m.verify)
}

func (ba *basicAuthServer) Dependencies() []component.ID {
	if ba.htpasswd != nil && ba.htpasswd.SecretProvider != nil {
		return []component.ID{ba.htpasswd.SecretProvider.ID}
	}
	return nil
}

func (ba *basicAuthServer) updateHtpasswd(raw string) error {
	htp, err := htpasswd.NewFromReader(strings.NewReader(raw), htpasswd.DefaultSystems, nil)
	if err != nil {
		return err
	}
	ba.matcher.Store(&htpasswdMatcher{htp: htp})
	return nil
}

var (
	_ extension.Extension            = (*basicAuthClient)(nil)
	_ extensionauth.HTTPClient       = (*basicAuthClient)(nil)
	_ extensionauth.GRPCClient       = (*basicAuthClient)(nil)
	_ extensioncapabilities.Dependent = (*basicAuthClient)(nil)
)

type secretCredentials struct {
	username string
	password string
}

type basicAuthClient struct {
	clientAuth       *ClientAuthSettings
	logger           *zap.Logger
	usernameResolver credentialsfile.ValueResolver
	passwordResolver credentialsfile.ValueResolver
	creds            atomic.Pointer[secretCredentials]
}

func (ba *basicAuthClient) Start(ctx context.Context, host component.Host) error {
	if ba.clientAuth == nil {
		return errNoCredentialSource
	}
	ca := ba.clientAuth

	if ca.SecretProvider != nil {
		cfg := ca.SecretProvider
		ext, ok := host.GetExtensions()[cfg.ID]
		if !ok {
			return fmt.Errorf("secret provider extension %q not found", cfg.ID)
		}
		sp, ok := ext.(secretprovider.SecretProvider)
		if !ok {
			return fmt.Errorf("extension %q does not implement SecretProvider", cfg.ID)
		}

		if err := ba.refreshFromSecretProvider(ctx, sp, cfg); err != nil {
			return fmt.Errorf("initial secret fetch from %q: %w", cfg.ID, err)
		}

		sp.OnChange(func(newValue string) {
			if err := ba.parseAndStoreCredentials(newValue, cfg); err != nil {
				ba.logger.Error("failed to parse credentials from secret", zap.Error(err))
				return
			}
			ba.logger.Info("credentials updated from secret provider")
		})
		return nil
	}

	if ca.Username != "" || ca.UsernameFile != "" {
		r, err := credentialsfile.NewValueResolver(ca.Username, ca.UsernameFile, ba.logger)
		if err != nil {
			return err
		}
		if err := r.Start(ctx); err != nil {
			return err
		}
		ba.usernameResolver = r
	}
	if string(ca.Password) != "" || ca.PasswordFile != "" {
		r, err := credentialsfile.NewValueResolver(string(ca.Password), ca.PasswordFile, ba.logger)
		if err != nil {
			return err
		}
		if err := r.Start(ctx); err != nil {
			return err
		}
		ba.passwordResolver = r
	}

	return nil
}

func (ba *basicAuthClient) Shutdown(_ context.Context) error {
	var errs []error
	if ba.usernameResolver != nil {
		errs = append(errs, ba.usernameResolver.Shutdown())
	}
	if ba.passwordResolver != nil {
		errs = append(errs, ba.passwordResolver.Shutdown())
	}
	return errors.Join(errs...)
}

func (ba *basicAuthClient) Username() string {
	if ba.usernameResolver != nil {
		return ba.usernameResolver.Value()
	}
	if ba.clientAuth != nil && ba.clientAuth.Username != "" {
		return ba.clientAuth.Username
	}
	if c := ba.creds.Load(); c != nil {
		return c.username
	}
	return ""
}

func (ba *basicAuthClient) Password() string {
	if ba.passwordResolver != nil {
		return ba.passwordResolver.Value()
	}
	if ba.clientAuth != nil && string(ba.clientAuth.Password) != "" {
		return string(ba.clientAuth.Password)
	}
	if c := ba.creds.Load(); c != nil {
		return c.password
	}
	return ""
}

func (ba *basicAuthClient) Dependencies() []component.ID {
	if ba.clientAuth != nil && ba.clientAuth.SecretProvider != nil {
		return []component.ID{ba.clientAuth.SecretProvider.ID}
	}
	return nil
}

func (ba *basicAuthClient) refreshFromSecretProvider(ctx context.Context, sp secretprovider.SecretProvider, cfg *SecretProviderConfig) error {
	raw, err := sp.GetSecret(ctx)
	if err != nil {
		return fmt.Errorf("get secret: %w", err)
	}
	return ba.parseAndStoreCredentials(raw, cfg)
}

func (ba *basicAuthClient) parseAndStoreCredentials(raw string, cfg *SecretProviderConfig) error {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("parse secret as JSON: %w", err)
	}
	uVal, ok := parsed[cfg.UsernameKey]
	if !ok {
		return fmt.Errorf("key %q not found in secret JSON", cfg.UsernameKey)
	}
	u, ok := uVal.(string)
	if !ok {
		return fmt.Errorf("key %q in secret is not a string", cfg.UsernameKey)
	}
	pVal, ok := parsed[cfg.PasswordKey]
	if !ok {
		return fmt.Errorf("key %q not found in secret JSON", cfg.PasswordKey)
	}
	p, ok := pVal.(string)
	if !ok {
		return fmt.Errorf("key %q in secret is not a string", cfg.PasswordKey)
	}
	ba.creds.Store(&secretCredentials{username: u, password: p})
	return nil
}

func (ba *basicAuthClient) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return basicauth.NewRoundTripper(base, ba)
}

func (ba *basicAuthClient) PerRPCCredentials() (creds.PerRPCCredentials, error) {
	return basicauth.NewPerRPCCredentials(ba)
}
