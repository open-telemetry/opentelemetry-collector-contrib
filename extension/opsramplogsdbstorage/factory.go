//go:build !freebsd

package opsramplogsdbstorage

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opsramplogsdbstorage/internal/metadata"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

// NewFactory creates a factory for DBStorage extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.LogsStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(
	_ context.Context,
	params extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	return newDBStorage(params.Logger, cfg.(*Config))
}
