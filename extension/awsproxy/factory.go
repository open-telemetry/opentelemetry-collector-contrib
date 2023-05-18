// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsproxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

const (
	defaultEndpoint = "0.0.0.0:2000"
)

// NewFactory creates a factory for awsproxy extension.
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
		ProxyConfig: proxy.Config{
			TCPAddr: confignet.TCPAddr{
				Endpoint: defaultEndpoint,
			},
			TLSSetting: configtls.TLSClientSetting{
				Insecure: false,
			},
		},
	}
}

func createExtension(_ context.Context, params extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	return newXrayProxy(cfg.(*Config), params.Logger)
}
