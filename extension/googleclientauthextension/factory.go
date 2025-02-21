// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googleclientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"

import (
	"context"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	eCfg := cfg.(*Config)
	return googleclientauthextension.CreateExtension(ctx, set, &eCfg.Config)
}

func createDefaultConfig() component.Config {
	cfg := googleclientauthextension.CreateDefaultConfig().(*googleclientauthextension.Config)
	return &Config{
		Config: *cfg,
	}
}
