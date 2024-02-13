// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension/internal/metadata"
)

const (
	defaultStorageType = "in-memory"
)

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
		StorageType: defaultStorageType,
	}
}

func createExtension(_ context.Context, set extension.CreateSettings, conf component.Config) (extension.Extension, error) {
	return nil, nil
}
