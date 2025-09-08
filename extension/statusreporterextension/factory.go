package statusreporterextension

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

// The statusreporter extension exposes health information about the collector
// at the "/api/otel-status" endpoint, accessible from other pods.
// It monitors the status of traces and metrics exporters and provides
// connection status information.
var (
	Type               = component.MustNewType("statusreporter")
	ExtensionStability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for the statusreporter extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		Type,
		createDefaultConfig,
		createExtension,
		ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		MetricsEndpoint: "http://localhost:8888/metrics",
		StaleThreshold:  300,
		EngineID:        "unknown",
		PodName:         "unknown",
		Port:            8080,
		AuthEnabled:     false,
		SecretValue:     "secret",
	}
}

func createExtension(
	_ context.Context,
	set extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	return newStatusReporterExtension(cfg.(*Config), set.TelemetrySettings), nil
}
