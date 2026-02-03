// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonfileobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/jsonfileobserver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/jsonfileobserver/internal/metadata"
)

// NewFactory creates a factory for the JSON file observer extension.
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
		RefreshInterval: 10 * time.Second,
	}
}

func createExtension(
	_ context.Context,
	set extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	return newObserver(set, cfg.(*Config))
}
