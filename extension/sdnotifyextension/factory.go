// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdnotifyextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sdnotifyextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sdnotifyextension/internal/metadata"
)

// NewFactory returns the factory for the sdnotify extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(
	_ context.Context,
	set extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	return newSDNotify(cfg.(*Config), set.Logger), nil
}
