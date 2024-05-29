// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

var providerFactories = map[string]provider.GeoIPProviderFactory{}

// NewFactory creates a new processor factory with default configuration,
// and registers the processors for metrics, traces, and logs.
func NewFactory() processor.Factory {
	return processor.NewFactory(metadata.Type, createDefaultConfig, processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability), processor.WithLogs(createLogsProcessor, metadata.LogsStability), processor.WithTraces(createTracesProcessor, metadata.TracesStability))
}

func getProviderFactory(key string) (provider.GeoIPProviderFactory, bool) {
	if factory, ok := providerFactories[key]; ok {
		return factory, true
	}

	return nil, false
}

// createDefaultConfig returns a default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{}
}

func createGeoIPProviders(
	ctx context.Context,
	set processor.CreateSettings,
	config *Config,
	factories map[string]provider.GeoIPProviderFactory,
) ([]provider.GeoIPProvider, error) {
	providers := make([]provider.GeoIPProvider, 0, len(config.Providers))

	for key, cfg := range config.Providers {
		factory := factories[key]
		if factory == nil {
			return nil, fmt.Errorf("geoIP provider factory not found for key: %q", key)
		}

		provider, err := factory.CreateGeoIPProvider(ctx, set, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider for key %q: %w", key, err)
		}

		providers = append(providers, provider)

	}

	return providers, nil
}

func createMetricsProcessor(ctx context.Context, set processor.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (processor.Metrics, error) {
	geoCfg := cfg.(*Config)
	providers, err := createGeoIPProviders(ctx, set, geoCfg, providerFactories)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetricsProcessor(ctx, set, cfg, nextConsumer, newGeoIPProcessor(providers).processMetrics, processorhelper.WithCapabilities(processorCapabilities))
}

func createTracesProcessor(ctx context.Context, set processor.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	geoCfg := cfg.(*Config)
	providers, err := createGeoIPProviders(ctx, set, geoCfg, providerFactories)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTracesProcessor(ctx, set, cfg, nextConsumer, newGeoIPProcessor(providers).processTraces, processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(ctx context.Context, set processor.CreateSettings, cfg component.Config, nextConsumer consumer.Logs) (processor.Logs, error) {
	geoCfg := cfg.(*Config)
	providers, err := createGeoIPProviders(ctx, set, geoCfg, providerFactories)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogsProcessor(ctx, set, cfg, nextConsumer, newGeoIPProcessor(providers).processLogs, processorhelper.WithCapabilities(processorCapabilities))
}
