// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonencodingextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(_ context.Context, _ extension.CreateSettings, _ component.Config) (extension.Extension, error) {
	return &jsonExtension{}, nil
}

func createDefaultConfig() component.Config {
	return &Config{}
}
