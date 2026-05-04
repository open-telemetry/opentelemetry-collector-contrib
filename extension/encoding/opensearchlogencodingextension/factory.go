// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchlogencodingextension // import "github.com/cloudoperators/opentelemetry-collector-contrib/extension/encoding/opensearchlogencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/cloudoperators/opentelemetry-collector-contrib/extension/encoding/opensearchlogencodingextension/internal/metadata"
)

// NewFactory creates a factory for the OpenSearch log encoding extension.
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

func createExtension(_ context.Context, _ extension.Settings, _ component.Config) (extension.Extension, error) {
	return &opensearchLogExtension{}, nil
}
