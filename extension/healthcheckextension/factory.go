// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

const defaultPort = 13133

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
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/",
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)

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
