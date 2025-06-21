package externalauthextension

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	typeStr   = "externalauth"
	stability = component.StabilityLevelAlpha
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		component.MustNewType("externalauth"),
		createDefaultConfig,
		createExtension,
		stability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint: "https://placeholder.com",
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newExternalAuth(cfg.(*Config), set.TelemetrySettings)
}
