// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var typeStr = component.MustNewType("resource_exhausted_retry")

// NewFactory creates a factory for the resourceexhaustedretryextension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		create,
		component.StabilityLevelDevelopment,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func create(_ context.Context, _ extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newExtension(cfg.(*Config)), nil
}
