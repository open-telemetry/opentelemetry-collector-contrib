// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

const defaultPort = 13133

// Feature gate that switches the extension to the shared healthcheck implementation
var disableCompatibilityWrapperGate = featuregate.GlobalRegistry().MustRegister(
	"extension.healthcheck.disableCompatibilityWrapper",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Switch to the shared healthcheck implementation powered by component status events"),
	featuregate.WithRegisterFromVersion("v0.138.0"),
	featuregate.WithRegisterToVersion("v0.143.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42256"),
)

// NewFactory creates a factory for HealthCheck extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		LegacyConfig: healthcheck.HTTPLegacyConfig{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: testutil.EndpointForPort(defaultPort),
			},
			Path: "/",
			CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
				Enabled:                  false,
				Interval:                 "5m",
				ExporterFailureThreshold: 5,
			},
		},
	}
}

func createExtension(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)

	if disableCompatibilityWrapperGate.IsEnabled() {
		// When feature gate is enabled, use v2 implementation directly.
		// The feature gate controls behavior, not the presence of v2 config fields.
		config.UseV2 = true

		// If no v2 config is set, create HTTP config from legacy settings for backward compatibility
		if config.HTTPConfig == nil && config.GRPCConfig == nil {
			config.HTTPConfig = &healthcheck.HTTPConfig{
				ServerConfig: config.ServerConfig,
				Status: healthcheck.PathConfig{
					Enabled: true,
					Path:    config.Path,
				},
				Config: healthcheck.PathConfig{
					Enabled: false,
					Path:    "/config",
				},
			}
		}

		return healthcheck.NewHealthCheckExtension(ctx, *config, set), nil
	}

	// Feature gate disabled: use legacy implementation.
	// V2 config fields (HTTPConfig/GRPCConfig) are ignored even if present in the config.
	// This allows users to have both configs present for easier migration.
	return newServer(*config, set.TelemetrySettings), nil
}
