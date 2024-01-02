// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarder // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder/internal/metadata"
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
	httpClientSettings := confighttp.NewDefaultHTTPClientSettings()
	httpClientSettings.Timeout = 10 * time.Second
	return &Config{
		Ingress: confighttp.HTTPServerSettings{
			Endpoint: defaultEndpoint,
		},
		Egress: httpClientSettings,
	}
}

func createExtension(
	_ context.Context,
	params extension.CreateSettings,
	cfg component.Config,
) (extension.Extension, error) {
	return newHTTPForwarder(cfg.(*Config), params.TelemetrySettings)
}
