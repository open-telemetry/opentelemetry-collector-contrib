package honeycombauthextension

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"os"
)

const typeStr = "honeycombauth"

func NewFactory() component.ExtensionFactory {
	return component.NewExtensionFactory(
		typeStr,
		createDefaultConfig,
		createExtension)
}

func createDefaultConfig() config.Extension {
	return &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
		Team:              os.Getenv(teamEnvKey),
		Dataset:           os.Getenv(datasetEnvKey),
	}
}

func createExtension(_ context.Context, _ component.ExtensionCreateSettings, cfg config.Extension) (component.Extension, error) {
	return newClientAuthenticator(cfg.(*Config)), nil
}
