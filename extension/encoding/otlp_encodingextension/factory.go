// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlp_encodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlp_encodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlp_encodingextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(_ context.Context, _ extension.CreateSettings, config component.Config) (extension.Extension, error) {
	return &otlpjsonencoding{
		config: config.(*Config),
	}, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		Protocol: OTLPProtocolJSON,
	}
}
