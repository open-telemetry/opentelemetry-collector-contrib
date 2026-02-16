// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

const defaultPort = 13133

// Feature gate that switches the extension to the shared healthcheck implementation
var useComponentStatusGate = featuregate.GlobalRegistry().MustRegister(
	"extension.healthcheck.useComponentStatus",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Switch to the shared healthcheck implementation powered by component status events"),
	featuregate.WithRegisterFromVersion("v0.142.0"),
	featuregate.WithRegisterToVersion("v0.147.0"),
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
		Config: healthcheck.Config{
			LegacyConfig: healthcheck.HTTPLegacyConfig{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: confignet.TransportTypeTCP,
						Endpoint:  testutil.EndpointForPort(defaultPort),
					},
				},
				Path: "/",
				CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
					Enabled:                  false,
					Interval:                 "5m",
					ExporterFailureThreshold: 5,
				},
			},
		},
	}
}

func createExtension(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)

	if useComponentStatusGate.IsEnabled() {
		// When feature gate is enabled, use v2 implementation directly.
		// The feature gate controls behavior, not the presence of v2 config fields.
		config.UseV2 = true

		// If no v2 config is set, create HTTP config from legacy settings for backward compatibility
		if config.HTTPConfig == nil && config.GRPCConfig == nil {
			set.Logger.Warn(
				"Feature gate enabled but using legacy config format. " +
					"Please migrate to v2 config format (http/grpc fields). " +
					"See: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/healthcheckextension#backward-compatibility",
			)
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

		return healthcheck.NewHealthCheckExtension(ctx, config.Config, set), nil
	}

	// Feature gate disabled: use legacy implementation.
	return newServer(*config, set.TelemetrySettings), nil
}
