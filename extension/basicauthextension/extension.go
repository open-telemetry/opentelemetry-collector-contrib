// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	creds "google.golang.org/grpc/credentials"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"
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
	if cfg.Htpasswd == nil || (cfg.Htpasswd.File == "" && cfg.Htpasswd.Inline == "") {
		return nil, errNoCredentialSource
	}

	return &basicAuthServer{
		htpasswd: cfg.Htpasswd,
	}, nil
}

var (
	_ extension.Extension  = (*basicAuthServer)(nil)
	_ extensionauth.Server = (*basicAuthServer)(nil)
)

type basicAuthServer struct {
	htpasswd  *HtpasswdSettings
	matchFunc func(username, password string) bool
	component.ShutdownFunc
}

func (ba *basicAuthServer) Start(_ context.Context, _ component.Host) error {
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

	ba.matchFunc = htp.Match

	return nil
}

func (ba *basicAuthServer) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	return basicauth.Authenticate(ctx, headers, ba.matchFunc)
}

var (
	_ extension.Extension      = (*basicAuthClient)(nil)
	_ extensionauth.HTTPClient = (*basicAuthClient)(nil)
	_ extensionauth.GRPCClient = (*basicAuthClient)(nil)
)

type basicAuthClient struct {
	clientAuth       *ClientAuthSettings
	logger           *zap.Logger
	usernameResolver credentialsfile.ValueResolver
	passwordResolver credentialsfile.ValueResolver
}

func (ba *basicAuthClient) Start(ctx context.Context, _ component.Host) error {
	if ba.clientAuth == nil {
		return errNoCredentialSource
	}
	ca := ba.clientAuth
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
	if ba.clientAuth != nil {
		return ba.clientAuth.Username
	}
	return ""
}

func (ba *basicAuthClient) Password() string {
	if ba.passwordResolver != nil {
		return ba.passwordResolver.Value()
	}
	if ba.clientAuth != nil {
		return string(ba.clientAuth.Password)
	}
	return ""
}

func (ba *basicAuthClient) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return basicauth.NewRoundTripper(base, ba)
}

func (ba *basicAuthClient) PerRPCCredentials() (creds.PerRPCCredentials, error) {
	return basicauth.NewPerRPCCredentials(ba)
}
