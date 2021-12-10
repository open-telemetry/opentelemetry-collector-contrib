package httpheaderextension

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/extensionhelper"
)

const (
	// The value of extension "type" in configuration.
	typeStr = "httpheader"
)

// NewFactory creates a factory for the OIDC Authenticator extension.
func NewFactory() component.ExtensionFactory {
	return extensionhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension)
}

func createDefaultConfig() config.Extension {
	return &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
	}
}

func createExtension(_ context.Context, set component.ExtensionCreateSettings, cfg config.Extension) (component.Extension, error) {
	return newExtension(cfg.(*Config), set.Logger)
}
