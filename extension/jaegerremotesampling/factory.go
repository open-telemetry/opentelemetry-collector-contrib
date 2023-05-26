// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/metadata"
)

const (
	// The value of extension "type" in configuration.
	typeStr = "jaegerremotesampling"
)

// NewFactory creates a factory for the OIDC Authenticator extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: &confighttp.HTTPServerSettings{
			Endpoint: ":5778",
		},
		GRPCServerSettings: &configgrpc.GRPCServerSettings{
			NetAddr: confignet.NetAddr{
				Endpoint:  ":14250",
				Transport: "tcp",
			},
		},
		Source: Source{},
	}
}

func createExtension(_ context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	return newExtension(cfg.(*Config), set.TelemetrySettings), nil
}
