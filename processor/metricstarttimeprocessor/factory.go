// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstarttimeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/subtractinitial"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/truereset"
)

// NewFactory creates a new metric start time processor factory.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability))
}

// createMetricsProcessor creates a metrics processor based on provided config.
func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	rCfg := cfg.(*Config)

	var adjustMetrics processorhelper.ProcessMetricsFunc

	switch rCfg.Strategy {
	case truereset.Type:
		adjuster := truereset.NewAdjuster(set.TelemetrySettings, rCfg.GCInterval)
		adjustMetrics = adjuster.AdjustMetrics
	case subtractinitial.Type:
		adjuster := subtractinitial.NewAdjuster(set.TelemetrySettings, rCfg.GCInterval)
		adjustMetrics = adjuster.AdjustMetrics
	}

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		adjustMetrics,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}
