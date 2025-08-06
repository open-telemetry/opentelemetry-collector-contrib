// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(_ context.Context, _ extension.Settings, config component.Config) (extension.Extension, error) {
	return &macosUnifiedLoggingExtension{
		config: config.(*Config),
	}, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		ParsePrivateLogs:      false,
		IncludeSignpostEvents: true,
		IncludeActivityEvents: true,
		MaxLogSize:            65536, // 64KB default
	}
}
