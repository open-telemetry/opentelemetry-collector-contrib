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
		ServerConfig: confighttp.ServerConfig{
			Endpoint: testutil.EndpointForPort(defaultPort),
		},
		Path:                   "/",
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
	}
}

func createExtension(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)

	if err := config.Validate(); err != nil {
		return nil, err
	}

	if disableCompatibilityWrapperGate.IsEnabled() {
		internalCfg := config.toInternalConfig(true)
		return healthcheck.NewHealthCheckExtension(ctx, internalCfg, set), nil
	}

	return newServer(*config, set.TelemetrySettings), nil
}

// defaultCheckCollectorPipelineSettings returns the default settings for CheckCollectorPipeline.
func defaultCheckCollectorPipelineSettings() checkCollectorPipelineSettings {
	return checkCollectorPipelineSettings{
		Enabled:                  false,
		Interval:                 "5m",
		ExporterFailureThreshold: 5,
	}
}
