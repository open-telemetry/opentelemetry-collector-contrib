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

// Feature gate to enable the compatibility wrapper that preserves v1 Ready/NotReady behavior
var useCompatibilityWrapperGate = featuregate.GlobalRegistry().MustRegister(
	"extension.healthcheck.useCompatibilityWrapper",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("Use compatibility wrapper to preserve v1 Ready/NotReady behavior when using shared healthcheck implementation"),
	featuregate.WithRegisterFromVersion("v0.135.0"),
	featuregate.WithRegisterToVersion("v0.140.0"),
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
	return &healthcheck.Config{
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
			UseV2: false,
		},
	}
}

func createExtension(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*healthcheck.Config)

	// Create the shared health check extension
	sharedExt := healthcheck.NewHealthCheckExtension(ctx, *config, set)

	// Conditionally wrap with compatibility layer based on feature gate
	if useCompatibilityWrapperGate.IsEnabled() {
		// Use compatibility wrapper to preserve v1 Ready/NotReady behavior
		return newCompatibilityWrapper(sharedExt), nil
	}

	// Use shared implementation directly (new behavior)
	return sharedExt, nil
}
