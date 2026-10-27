// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

// newMetricsProcessor builds a metrics processor that enriches each resource with
// ECS metadata. processorhelper wraps the enrichment function with the standard
// capabilities and Start/Shutdown lifecycle used across the collector.
func newMetricsProcessor(ctx context.Context, set processor.Settings, cfg *Config, next consumer.Metrics, endpoints endpointsFn) (processor.Metrics, error) {
	core, err := newCore(set.Logger, cfg, endpoints)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetrics(
		ctx, set, cfg, next,
		core.processMetrics,
		processorhelper.WithCapabilities(core.Capabilities()),
		processorhelper.WithStart(core.Start),
		processorhelper.WithShutdown(core.Shutdown),
	)
}

func (e *ecsCore) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := range rms.Len() {
		e.enrichResource(ctx, rms.At(i).Resource())
	}
	return md, nil
}
