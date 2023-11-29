package solarwindsapmsettingsextension

import (
	"context"
	"github.com/solarwinds/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	DefaultEndpoint = "apm.collector.cloud.solarwinds.com:443"
	DefaultInterval = "1m"
)

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint: DefaultEndpoint,
		Interval: DefaultInterval,
	}
}

func createExtension(_ context.Context, settings extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	return newSolarwindsApmSettingsExtension(cfg.(*Config), settings.Logger)
}

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}
