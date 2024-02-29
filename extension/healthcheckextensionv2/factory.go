// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package healthcheckextensionv2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/metadata"
)

const (
	// Use 0.0.0.0 to make the health check endpoint accessible
	// in container orchestration environments like Kubernetes.
	defaultGRPCEndpoint = "0.0.0.0:13132"
	defaultHTTPEndpoint = "0.0.0.0:13133"
)

// NewFactory creates a factory for HealthCheck extension.
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
		LegacySettings: http.LegacySettings{
			HTTPServerSettings: confighttp.HTTPServerSettings{
				Endpoint: defaultHTTPEndpoint,
			},
			Path: "/",
		},
		HTTPSettings: &http.Settings{
			HTTPServerSettings: confighttp.HTTPServerSettings{
				Endpoint: defaultHTTPEndpoint,
			},
			Status: http.PathSettings{
				Enabled: true,
				Path:    "/status",
			},
			Config: http.PathSettings{
				Enabled: false,
				Path:    "/config",
			},
		},
		GRPCSettings: &grpc.Settings{
			GRPCServerSettings: configgrpc.GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  defaultGRPCEndpoint,
					Transport: "tcp",
				},
			},
		},
	}
}

func createExtension(ctx context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	return newExtension(ctx, *config, set), nil
}
