// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(_ context.Context, settings extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)

	return &azureExtension{
		config: config,
		traceUnmarshaler: azure.TracesUnmarshaler{
			Version:     settings.BuildInfo.Version,
			Logger:      settings.Logger,
			TimeFormats: config.Traces.TimeFormats,
		},
		logUnmarshaler: azurelogs.ResourceLogsUnmarshaler{
			Version:     settings.BuildInfo.Version,
			Logger:      settings.Logger,
			TimeFormats: config.Logs.TimeFormats,
		},
	}, nil
}

func createDefaultConfig() component.Config {
	return &Config{}
}
