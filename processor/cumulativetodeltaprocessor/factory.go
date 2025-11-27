// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

var defaultMaxStalenessFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"processor.cumulativetodelta.defaultmaxstaleness",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, max_staleness defaults to 1 hour instead of 0 (infinite retention). This helps prevent unbounded memory growth in long-running collector instances."),
	featuregate.WithRegisterFromVersion("v0.141.0"),
)

// NewFactory returns a new factory for the Metrics Generation processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := &Config{}
	if defaultMaxStalenessFeatureGate.IsEnabled() {
		cfg.MaxStaleness = 1 * time.Hour
	}
	return cfg
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("configuration parsing error")
	}

	metricsProcessor, err := newCumulativeToDeltaProcessor(processorConfig, set.Logger)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		metricsProcessor.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithShutdown(metricsProcessor.shutdown))
}
