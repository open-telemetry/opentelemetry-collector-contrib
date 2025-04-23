// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogfleetautomationextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogfleetautomationextension"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogfleetautomationextension/internal/httpserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogfleetautomationextension/internal/metadata"
)

// NewFactory creates a factory for the Datadog Fleet Automation extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		create,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig: confighttp.NewDefaultClientConfig(),
		API: APIConfig{
			Site: defaultSite,
		},
		HTTPConfig: &httpserver.Config{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: httpserver.DefaultServerEndpoint,
			},
			Path: "/metadata",
		},
	}
}

func create(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	extensionConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}
	faext, err := newExtension(ctx, extensionConfig, set)
	if err != nil {
		return nil, err
	}
	return faext, nil
}
