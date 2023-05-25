// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

// The value of extension "type" in configuration.
const typeStr component.Type = "db_storage"

// NewFactory creates a factory for DBStorage extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelAlpha,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(
	_ context.Context,
	params extension.CreateSettings,
	cfg component.Config,
) (extension.Extension, error) {
	return newDBStorage(params.Logger, cfg.(*Config))
}
