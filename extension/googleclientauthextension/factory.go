// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package googleclientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	eCfg := cfg.(*Config)
	return clientauth.CreateExtension(ctx, set, &eCfg.Config)
}

func createDefaultConfig() component.Config {
	cfg := clientauth.CreateDefaultConfig().(*clientauth.Config)
	return &Config{
		Config: *cfg,
	}
}
