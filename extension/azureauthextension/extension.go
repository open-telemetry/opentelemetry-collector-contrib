// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension"

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/extension/extensionauth"
	"google.golang.org/grpc/credentials"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

type authenticator struct {
	credential azcore.TokenCredential
	logger     *zap.Logger
}

var (
	_ extension.Extension      = (*authenticator)(nil)
	_ extensionauth.HTTPClient = (*authenticator)(nil)
	_ extensionauth.Server     = (*authenticator)(nil)
)

func newAzureAuthenticator(cfg *Config, logger *zap.Logger) (*authenticator, error) {
	var credential azcore.TokenCredential
	if cfg.UseDefault {
		// TODO
	}
	if cfg.Workload != nil {
		// TODO
	}
	if cfg.Managed != nil {
		// TODO
	}
	if cfg.ServicePrincipal != nil {
		// TODO
	}
	return &authenticator{
		credential: credential,
		logger:     logger,
	}, nil
}

func (a authenticator) Start(ctx context.Context, host component.Host) error {
	// TODO
	return nil
}

func (a authenticator) Shutdown(ctx context.Context) error {
	// TODO
	return nil
}

func (a authenticator) Authenticate(ctx context.Context, sources map[string][]string) (context.Context, error) {
	// TODO
	return ctx, nil
}

func (a authenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	// TODO
	return nil, nil
}

func (a authenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	// TODO
	return nil, nil
}
