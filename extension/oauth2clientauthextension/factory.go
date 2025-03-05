// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension/internal/metadata"
)

// NewFactory creates a factory for the oauth2 client Authenticator extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ExpiryBuffer: 5 * time.Minute,
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	ca, err := newClientAuthenticator(cfg.(*Config), set.Logger)
	if err != nil {
		return nil, err
	}

	return extensionauth.NewClient(
		extensionauth.WithClientRoundTripper(ca.roundTripper),
		extensionauth.WithClientPerRPCCredentials(ca.perRPCCredentials),
	)
}
