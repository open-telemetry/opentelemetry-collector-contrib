// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Leaf Span Pruning processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		MinSpansToAggregate:        2,
		SummarySpanNameSuffix:      "_aggregated",
		AggregationAttributePrefix: "aggregation.",
		AggregationHistogramBuckets: []time.Duration{
			5 * time.Millisecond,
			10 * time.Millisecond,
			25 * time.Millisecond,
			50 * time.Millisecond,
			100 * time.Millisecond,
			250 * time.Millisecond,
			500 * time.Millisecond,
			time.Second,
			2500 * time.Millisecond,
			5 * time.Second,
			10 * time.Second,
		},
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	pCfg := cfg.(*Config)

	p, err := newLeafSpanPruningProcessor(set, pCfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		p.processTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}
