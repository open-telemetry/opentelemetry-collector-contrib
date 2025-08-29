// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension/internal/metadata"
)

// NewFactory creates a new factory for the macOS Unified Logging encoding extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

// createExtension creates a new instance of the macOS Unified Logging encoding extension.
func createExtension(_ context.Context, _ extension.Settings, config component.Config) (extension.Extension, error) {
	return &MacosUnifiedLoggingExtension{
		config: config.(*Config),
	}, nil
}

// createDefaultConfig creates the default configuration for the extension.
func createDefaultConfig() component.Config {
	return &Config{
		DebugMode: false,
	}
}
