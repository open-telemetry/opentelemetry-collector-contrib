// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/remotetapextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/remotetapextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelDevelopment,
	)
}

func createExtension(_ context.Context, settings extension.CreateSettings, config component.Config) (extension.Extension, error) {
	return &remoteObserverExtension{config: config.(*Config), settings: settings}, nil
}
