// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension"

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
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

func newAzureAuthenticator(_ *Config, logger *zap.Logger) *authenticator {
	var credential azcore.TokenCredential
	// TODO
	// if cfg.UseDefault {
	// }
	// if cfg.Workload != nil {
	// }
	// if cfg.Managed != nil {
	// }
	// if cfg.ServicePrincipal != nil {
	// }
	return &authenticator{
		credential: credential,
		logger:     logger,
	}
}

func (a authenticator) Start(_ context.Context, _ component.Host) error {
	// TODO
	return nil
}

func (a authenticator) Shutdown(_ context.Context) error {
	// TODO
	return nil
}

func (a authenticator) Authenticate(ctx context.Context, _ map[string][]string) (context.Context, error) {
	// TODO
	return ctx, nil
}

func (a authenticator) RoundTripper(_ http.RoundTripper) (http.RoundTripper, error) {
	// TODO
	return nil, nil
}
