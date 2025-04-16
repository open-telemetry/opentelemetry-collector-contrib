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
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
	maxmind "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider/maxmindprovider"
)

var (
	processorCapabilities = consumer.Capabilities{MutatesData: true}
	// defaultAttributes holds a list of default resource attribute keys.
	// These keys are used to identify an IP address attribute associated with the resource.
	defaultAttributes = []attribute.Key{
		// The client attributes are in use by the HTTP semantic conventions
		semconv.ClientAddressKey,
		// The source attributes are used when there is no client/server relationship between the two sides, or when that relationship is unknown
		semconv.SourceAddressKey,
	}
)

// providerFactories is a map that stores GeoIPProviderFactory instances, keyed by the provider type.
var providerFactories = map[string]provider.GeoIPProviderFactory{
	maxmind.TypeStr: &maxmind.Factory{},
}

// NewFactory creates a new processor factory with default configuration,
// and registers the processors for metrics, traces, and logs.
func NewFactory() processor.Factory {
	return processor.NewFactory(metadata.Type, createDefaultConfig, processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability), processor.WithLogs(createLogsProcessor, metadata.LogsStability), processor.WithTraces(createTracesProcessor, metadata.TracesStability))
}

// getProviderFactory retrieves the GeoIPProviderFactory for the given key.
// It returns the factory and a boolean indicating whether the factory was found.
func getProviderFactory(key string) (provider.GeoIPProviderFactory, bool) {
	if factory, ok := providerFactories[key]; ok {
		return factory, true
	}

	return nil, false
}

// createDefaultConfig returns a default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		Context:    resource,
		Attributes: defaultAttributes,
	}
}

// createGeoIPProviders creates a list of GeoIPProvider instances based on the provided configuration and providers factories.
func createGeoIPProviders(
	ctx context.Context,
	set processor.Settings,
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

func createMetricsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Metrics) (processor.Metrics, error) {
	geoCfg := cfg.(*Config)
	providers, err := createGeoIPProviders(ctx, set, geoCfg, providerFactories)
	if err != nil {
		return nil, err
	}
	geoProcessor := newGeoIPProcessor(geoCfg, providers, set)
	return processorhelper.NewMetrics(ctx, set, cfg, nextConsumer, geoProcessor.processMetrics, processorhelper.WithShutdown(geoProcessor.shutdown), processorhelper.WithCapabilities(processorCapabilities))
}

func createTracesProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	geoCfg := cfg.(*Config)
	providers, err := createGeoIPProviders(ctx, set, geoCfg, providerFactories)
	if err != nil {
		return nil, err
	}
	geoProcessor := newGeoIPProcessor(geoCfg, providers, set)
	return processorhelper.NewTraces(ctx, set, cfg, nextConsumer, geoProcessor.processTraces, processorhelper.WithShutdown(geoProcessor.shutdown), processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Logs) (processor.Logs, error) {
	geoCfg := cfg.(*Config)
	providers, err := createGeoIPProviders(ctx, set, geoCfg, providerFactories)
	if err != nil {
		return nil, err
	}
	geoProcessor := newGeoIPProcessor(geoCfg, providers, set)
	return processorhelper.NewLogs(ctx, set, cfg, nextConsumer, geoProcessor.processLogs, processorhelper.WithShutdown(geoProcessor.shutdown), processorhelper.WithCapabilities(processorCapabilities))
}
