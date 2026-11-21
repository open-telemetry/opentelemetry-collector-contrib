// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalstate // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/signalstate"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/signalstate/internal/metadata"
)

// NewFactory creates the signalstate extension factory.
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
	ctx context.Context,
	set extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	c := cfg.(*Config)
	return newSignalState(ctx, c, set)
}
