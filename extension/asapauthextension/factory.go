// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package asapauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension/internal/metadata"
)

// NewFactory creates a factory for asapauthextension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(_ context.Context, _ extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	return createASAPClientAuthenticator(cfg.(*Config))
}

func createDefaultConfig() component.Config {
	return &Config{}
}
