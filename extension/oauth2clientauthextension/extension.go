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
	_ extension.Extension      = (clientAuthenticator)(nil)
	_ extensionauth.HTTPClient = (clientAuthenticator)(nil)
	_ extensionauth.GRPCClient = (clientAuthenticator)(nil)
)

// clientAuthenticator provides implementation for providing client authentication using OAuth2 client credentials
// workflow for both gRPC and HTTP clients.
type clientAuthenticator interface {
	RoundTripper(base http.RoundTripper) (http.RoundTripper, error)
	PerRPCCredentials() (credentials.PerRPCCredentials, error)
	Transport() http.RoundTripper
	Shutdown(ctx context.Context) error
	Start(ctx context.Context, host component.Host) error
}
type errorWrappingTokenSource struct {
	ts       oauth2.TokenSource
	tokenURL string
}

// errorWrappingTokenSource implements TokenSource
var _ oauth2.TokenSource = (*errorWrappingTokenSource)(nil)

// errFailedToGetSecurityToken indicates a problem communicating with OAuth2 server.
var errFailedToGetSecurityToken = errors.New("failed to get security token from token endpoint")

func newClientAuthenticator(cfg *Config, logger *zap.Logger) (clientAuthenticator, error) {
	if cfg.getMode() == "sts" {
		return newStsClientAuthenticator(cfg, logger)
	}
	return newTwoLeggedClientAuthenticator(cfg, logger)
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
