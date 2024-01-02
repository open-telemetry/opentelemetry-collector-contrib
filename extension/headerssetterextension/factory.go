// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/metadata"
)

// NewFactory creates a factory for the headers setter extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(
	_ context.Context,
	settings extension.CreateSettings,
	cfg component.Config,
) (extension.Extension, error) {
	return newHeadersSetterExtension(cfg.(*Config), settings.Logger)
}
