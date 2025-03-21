// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpmextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tpmextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	Type      = component.MustNewType("tpm")
	ScopeName = "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tpmextension"
)

const (
	ExtensionStability = component.StabilityLevelDevelopment
)

func createExtension(_ context.Context, settings extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newTPMExtension(cfg.(*Config), settings)
}

func NewFactory() extension.Factory {
	return extension.NewFactory(
		Type,
		createDefaultConfig,
		createExtension,
		ExtensionStability,
	)
}
