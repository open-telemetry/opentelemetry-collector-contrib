// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

var ()

// clientAuthenticator provides implementation for providing client authentication using OAuth2 client credentials
// workflow for both gRPC and HTTP clients.
type clientAuthenticator interface {
	extension.Extension
	extensionauth.HTTPClient
	extensionauth.GRPCClient
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
	if cfg.AuthMode == "sts" {
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
