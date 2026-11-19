// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

// newLogsProcessor builds a logs processor that enriches each resource with ECS
// metadata. processorhelper wraps the enrichment function with the standard
// capabilities and Start/Shutdown lifecycle used across the collector.
func newLogsProcessor(ctx context.Context, set processor.Settings, cfg *Config, next consumer.Logs, endpoints endpointsFn) (processor.Logs, error) {
	core, err := newCore(set.Logger, cfg, endpoints)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogs(
		ctx, set, cfg, next,
		core.processLogs,
		processorhelper.WithCapabilities(core.Capabilities()),
		processorhelper.WithStart(core.Start),
		processorhelper.WithShutdown(core.Shutdown),
	)
}

func (e *ecsCore) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rls := ld.ResourceLogs()
	for i := range rls.Len() {
		e.enrichResource(ctx, rls.At(i).Resource())
	}
	return ld, nil
}
