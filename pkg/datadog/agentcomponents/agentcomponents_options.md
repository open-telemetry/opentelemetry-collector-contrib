This file provides examples of how to use the NewConfigComponent options pattern for modules that need Datadog agent configurations.

```
package agentcomponents

import (
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// Example 1: A metrics-only module that only needs API configuration
func exampleMetricsOnlyModule(_ component.TelemetrySettings, apiKey configopaque.String) {
	config := NewConfigComponent(
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  apiKey,
				Site: "datadoghq.com",
			},
		}),
		WithCustomConfig("metrics.enabled", true, pkgconfigmodel.SourceFile),
		WithCustomConfig("metrics.batch_size", 100, pkgconfigmodel.SourceDefault),
	)
	_ = config // Use the config component for metrics setup
}

// Example 2: A traces module with custom configuration
func exampleTracesModule(set component.TelemetrySettings, cfg *datadogconfig.Config) {
	config := NewConfigComponent(
		WithAPIConfig(cfg),
		WithLogLevel(set),
		WithCustomConfig("traces.enabled", true, pkgconfigmodel.SourceFile),
		WithCustomConfig("traces.sample_rate", 0.1, pkgconfigmodel.SourceDefault),
		WithCustomConfig("traces.max_traces_per_second", 1000, pkgconfigmodel.SourceDefault),
		WithProxyFromEnv(),
	)
	_ = config // Use the config component for traces setup
}

// Example 3: A full-featured logs module using all logs options
func exampleLogsModule(set component.TelemetrySettings, cfg *datadogconfig.Config) {
	config := NewConfigComponent(
		WithAPIConfig(cfg),
		WithLogLevel(set),
		WithLogsConfig(cfg),
		WithLogsDefaults(),
		WithProxyFromEnv(),
		// Additional logs-specific configuration
		WithCustomConfig("logs.dev_mode", false, pkgconfigmodel.SourceDefault),
		WithCustomConfig("logs.extra_metadata", true, pkgconfigmodel.SourceFile),
	)
	_ = config // Use the config component for logs setup
}

// Example 4: A minimal configuration for a custom connector/processor
func exampleMinimalModule(_ component.TelemetrySettings, apiKey configopaque.String) {
	config := NewConfigComponent(
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  apiKey,
				Site: "datadoghq.eu", // Different site
			},
		}),
		// Only set what's needed for this module
		WithCustomConfig("connector.buffer_size", 500, pkgconfigmodel.SourceDefault),
	)
	_ = config // Use the config component for connector setup
}

// Example 5: Advanced usage with conditional options
func exampleConditionalModule(set component.TelemetrySettings, cfg *datadogconfig.Config, enableAdvanced bool) {
	options := []ConfigOption{
		WithAPIConfig(cfg),
		WithLogLevel(set),
	}

	// Conditionally add more options based on configuration
	if enableAdvanced {
		options = append(options,
			WithLogsDefaults(),
			WithProxyFromEnv(),
			WithCustomConfig("advanced.feature_flag", true, pkgconfigmodel.SourceFile),
		)
	}

	// Add development-specific options in dev environments
	if cfg.API.Site == "datadoghq.local" {
		options = append(options,
			WithCustomConfig("dev.debug_mode", true, pkgconfigmodel.SourceDefault),
			WithCustomConfig("dev.verbose_logging", true, pkgconfigmodel.SourceDefault),
		)
	}

	config := NewConfigComponent(options...)
	_ = config // Use the config component
}

// Example 6: Creating reusable option sets for common patterns
var (
	// Common options for production environments
	ProductionOptions = []ConfigOption{
		WithLogsDefaults(),
		WithProxyFromEnv(),
	}

	// Common options for development environments
	DevelopmentOptions = []ConfigOption{
		WithCustomConfig("dev.debug_mode", true, pkgconfigmodel.SourceDefault),
		WithCustomConfig("dev.mock_endpoints", true, pkgconfigmodel.SourceDefault),
	}
)

func exampleReusableOptions(set component.TelemetrySettings, cfg *datadogconfig.Config, isProd bool) {
	baseOptions := []ConfigOption{
		WithAPIConfig(cfg),
		WithLogLevel(set),
	}

	var envOptions []ConfigOption
	if isProd {
		envOptions = ProductionOptions
	} else {
		envOptions = DevelopmentOptions
	}

	// Combine base options with environment-specific options
	allOptions := append(baseOptions, envOptions...)
	config := NewConfigComponent(allOptions...)
	_ = config // Use the config component
}
```
