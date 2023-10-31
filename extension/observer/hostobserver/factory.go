// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver/internal/metadata"
)

const (
	defaultCollectionInterval = 10
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
		RefreshInterval: defaultCollectionInterval * time.Second,
	}
}

func createExtension(
	_ context.Context,
	params extension.CreateSettings,
	cfg component.Config,
) (extension.Extension, error) {
	return newObserver(params, cfg.(*Config))
}
