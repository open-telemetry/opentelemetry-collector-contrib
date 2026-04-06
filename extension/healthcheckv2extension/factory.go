// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package healthcheckv2extension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

// NewFactory creates a factory for HealthCheck extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		healthcheck.NewDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return healthcheck.NewDefaultConfig()
}

func createExtension(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	return healthcheck.NewHealthCheckExtension(ctx, *config, set), nil
}
