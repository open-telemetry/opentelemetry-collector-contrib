// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension/internal/metadata"
)

const (
	defaultEndpoint = "localhost:1777"
)

// NewFactory creates a factory for pprof extension.
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
		TCPAddr: confignet.TCPAddr{
			Endpoint: defaultEndpoint,
		},
	}
}

func createExtension(_ context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	if config.TCPAddr.Endpoint == "" {
		return nil, errors.New("\"endpoint\" is required when using the \"pprof\" extension")
	}

	return newServer(*config, set.Logger), nil
}
