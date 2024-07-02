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
	defaultEndpoint           = "unix:///var/vcap/data/garden/garden.sock"
)

// NewFactory creates a factory for HostObserver extension.
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
		RefreshInterval: defaultCollectionInterval,
		Endpoint:        defaultEndpoint,
	}
}

func createExtension(
	_ context.Context,
	params extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	return newObserver(params, cfg.(*Config))
}
