// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	extensionName  = "otlpencoding"
	stabilityLevel = component.StabilityLevelDevelopment
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		extensionName,
		createDefaultConfig,
		createExtension,
		stabilityLevel,
	)
}

func createExtension(_ context.Context, _ extension.CreateSettings, config component.Config) (extension.Extension, error) {
	return newExtension(config.(*Config))
}

func createDefaultConfig() component.Config {
	return &Config{Protocol: otlpProto}
}
