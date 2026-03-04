// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarderextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarderextension"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarderextension/internal/metadata"
)

const (
	// Default endpoints to bind to.
	defaultEndpoint = ":6060"
)

// NewFactory creates a factory for HostObserver extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability)
}

func createDefaultConfig() component.Config {
	httpClientSettings := confighttp.NewDefaultClientConfig()
	httpClientSettings.Timeout = 10 * time.Second
	return &Config{
		Ingress: confighttp.ServerConfig{
			Endpoint: defaultEndpoint,
		},
		Egress: httpClientSettings,
	}
}

func createExtension(
	_ context.Context,
	params extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	return newHTTPForwarder(cfg.(*Config), params.TelemetrySettings)
}
