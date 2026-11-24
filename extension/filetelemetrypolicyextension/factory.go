// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filetelemetrypolicyextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/filetelemetrypolicyextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/filetelemetrypolicyextension/internal/metadata"
)

// NewFactory creates a factory for the file_telemetry_policy extension.
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

func createExtension(_ context.Context, settings extension.Settings, config component.Config) (extension.Extension, error) {
	return newExtension(config.(*Config), settings), nil
}
