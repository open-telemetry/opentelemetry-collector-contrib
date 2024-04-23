// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusapiserverextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/prometheusapiserverextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/prometheusapiserverextension/internal/metadata"
)

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

func createExtension(_ context.Context, settings extension.CreateSettings, config component.Config) (extension.Extension, error) {
	return &prometheusAPIServerExtension{config: config.(*Config), settings: settings}, nil
}
