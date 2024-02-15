// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension/internal/metadata"
)

var defaultStorageType = (*component.ID)(nil)

// NewFactory creates a factory for ack extension.
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
		StorageID: defaultStorageType,
	}
}

func createExtension(_ context.Context, _ extension.CreateSettings, _ component.Config) (extension.Extension, error) {
	return nil, nil
}
