// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package asapauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/xextension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension/internal/metadata"
)

// NewFactory creates a factory for asapauthextension.
func NewFactory() extension.Factory {
	return xextension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
		xextension.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createExtension(_ context.Context, _ extension.Settings, cfg component.Config) (extension.Extension, error) {
	return createASAPClientAuthenticator(cfg.(*Config))
}

func createDefaultConfig() component.Config {
	return &Config{}
}
