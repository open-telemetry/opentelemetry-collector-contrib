// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/metadata"
)

// NewFactory creates a factory for ECSObserver extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	cfg := DefaultConfig()
	return &cfg
}

func createExtension(ctx context.Context, params extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	sdCfg := cfg.(*Config)
	fetcher, err := newTaskFetcherFromConfig(*sdCfg, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("init fetcher failed: %w", err)
	}
	return createExtensionWithFetcher(params, sdCfg, fetcher)
}

// fetcher is mock in unit test or AWS API client
func createExtensionWithFetcher(params extension.CreateSettings, sdCfg *Config, fetcher *taskFetcher) (extension.Extension, error) {
	sd, err := newDiscovery(*sdCfg, serviceDiscoveryOptions{Logger: params.Logger, Fetcher: fetcher})
	if err != nil {
		return nil, err
	}
	return &ecsObserver{
		logger: params.Logger,
		sd:     sd,
	}, nil
}
