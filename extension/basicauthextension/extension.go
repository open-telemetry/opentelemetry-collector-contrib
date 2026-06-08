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
	"sync/atomic"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	creds "google.golang.org/grpc/credentials"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/awssecretsmanager"
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
	if cfg.Htpasswd == nil || (cfg.Htpasswd.File == "" && cfg.Htpasswd.Inline == "" && cfg.Htpasswd.AWSSecret == nil) {
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
	htpasswd    *HtpasswdSettings
	logger      *zap.Logger
	matchFunc   atomic.Pointer[func(string, string) bool]
	awsResolver *awssecretsmanager.Resolver
}

func (ba *basicAuthServer) Start(ctx context.Context, _ component.Host) error {
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

	matchFn := htp.Match
	ba.matchFunc.Store(&matchFn)

	if ba.htpasswd.AWSSecret != nil {
		cfg := ba.htpasswd.AWSSecret
		resolver := awssecretsmanager.NewResolver(
			cfg.SecretARN,
			cfg.Region,
			cfg.ValueKey,
			cfg.RefreshInterval,
			ba.logger,
			func(newValue string) {
				htp, err := htpasswd.NewFromReader(strings.NewReader(newValue), htpasswd.DefaultSystems, nil)
				if err != nil {
					ba.logger.Error("failed to rebuild htpasswd matcher after secret rotation", zap.Error(err))
					return
				}
				fn := htp.Match
				ba.matchFunc.Store(&fn)
			},
		)

		if err := resolver.Start(ctx); err != nil {
			return err
		}
		ba.awsResolver = resolver

		htp, err := htpasswd.NewFromReader(strings.NewReader(resolver.Value()), htpasswd.DefaultSystems, nil)
		if err != nil {
			return fmt.Errorf("parse htpasswd from AWS secret: %w", err)
		}
		fn := htp.Match
		ba.matchFunc.Store(&fn)
	}

	return nil
}

func (ba *basicAuthServer) Shutdown(_ context.Context) error {
	if ba.awsResolver != nil {
		return ba.awsResolver.Shutdown()
	}
	return nil
}

func (ba *basicAuthServer) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	return basicauth.Authenticate(ctx, headers, *ba.matchFunc.Load())
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

	if ca.AWSSecret != nil {
		cfg := ca.AWSSecret

		usernameResolver := awssecretsmanager.NewResolver(
			cfg.SecretARN, cfg.Region, cfg.UsernameKey, cfg.RefreshInterval, ba.logger, nil,
		)
		if err := usernameResolver.Start(ctx); err != nil {
			return fmt.Errorf("start username resolver: %w", err)
		}
		ba.usernameResolver = usernameResolver

		passwordResolver := awssecretsmanager.NewResolver(
			cfg.SecretARN, cfg.Region, cfg.PasswordKey, cfg.RefreshInterval, ba.logger, nil,
		)
		if err := passwordResolver.Start(ctx); err != nil {
			_ = usernameResolver.Shutdown()
			return fmt.Errorf("start password resolver: %w", err)
		}
		ba.passwordResolver = passwordResolver
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
