// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cfgardenobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver/internal/metadata"
)

const (
	defaultCollectionInterval = 1 * time.Minute
	defaultCacheSyncInterval  = 5 * time.Minute
	defaultEndpoint           = "/var/vcap/data/garden/garden.sock"
)

// NewFactory creates a factory for CfGardenObserver extension.
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
		RefreshInterval:   defaultCollectionInterval,
		CacheSyncInterval: defaultCacheSyncInterval,
		Garden: GardenConfig{
			Endpoint: defaultEndpoint,
		},
	}
}

func createExtension(
	_ context.Context,
	settings extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	return newObserver(cfg.(*Config), settings.Logger)
}
