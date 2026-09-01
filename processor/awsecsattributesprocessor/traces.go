// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

// newTracesProcessor builds a traces processor that enriches each resource with
// ECS metadata. processorhelper wraps the enrichment function with the standard
// capabilities and Start/Shutdown lifecycle used across the collector.
func newTracesProcessor(ctx context.Context, set processor.Settings, cfg *Config, next consumer.Traces, endpoints endpointsFn) (processor.Traces, error) {
	core, err := newCore(set.Logger, cfg, endpoints)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTraces(
		ctx, set, cfg, next,
		core.processTraces,
		processorhelper.WithCapabilities(core.Capabilities()),
		processorhelper.WithStart(core.Start),
		processorhelper.WithShutdown(core.Shutdown),
	)
}

func (e *ecsCore) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := range rss.Len() {
		e.enrichResource(ctx, rss.At(i).Resource())
	}
	return td, nil
}
