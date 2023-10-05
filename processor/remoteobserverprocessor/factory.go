// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remoteobserverprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/remoteobserverprocessor"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/remoteobserverprocessor/internal/metadata"
)

// onceLogLocalHost is used to log the info log about changing the default once.
var onceLogLocalHost sync.Once

var processors = sharedcomponent.NewSharedComponents()

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTraceProcessor, metadata.TracesStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createMetricsProcessor(ctx context.Context, params processor.CreateSettings, cfg component.Config, c consumer.Metrics) (processor.Metrics, error) {
	onceLogLocalHost.Do(func() {
		component.LogAboutUseLocalHostAsDefault(params.Logger)
	})
	rCfg := cfg.(*Config)
	p := processors.GetOrAdd(cfg, func() component.Component {
		return newProcessor(params, rCfg)
	})
	fn := p.Unwrap().(*wsprocessor).ConsumeMetrics
	return processorhelper.NewMetricsProcessor(ctx, params, cfg, c,
		fn,
		processorhelper.WithCapabilities(consumer.Capabilities{
			MutatesData: false,
		}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown))
}

func createLogsProcessor(ctx context.Context, params processor.CreateSettings, cfg component.Config, c consumer.Logs) (processor.Logs, error) {
	onceLogLocalHost.Do(func() {
		component.LogAboutUseLocalHostAsDefault(params.Logger)
	})
	rCfg := cfg.(*Config)
	p := processors.GetOrAdd(cfg, func() component.Component {
		return newProcessor(params, rCfg)
	})
	fn := p.Unwrap().(*wsprocessor).ConsumeLogs
	return processorhelper.NewLogsProcessor(ctx, params, cfg, c,
		fn,
		processorhelper.WithCapabilities(consumer.Capabilities{
			MutatesData: false,
		}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown))
}

func createTraceProcessor(ctx context.Context, params processor.CreateSettings, cfg component.Config, c consumer.Traces) (processor.Traces, error) {
	onceLogLocalHost.Do(func() {
		component.LogAboutUseLocalHostAsDefault(params.Logger)
	})
	rCfg := cfg.(*Config)
	p := processors.GetOrAdd(cfg, func() component.Component {
		return newProcessor(params, rCfg)
	})
	fn := p.Unwrap().(*wsprocessor).ConsumeTraces
	return processorhelper.NewTracesProcessor(ctx, params, cfg, c,
		fn,
		processorhelper.WithCapabilities(consumer.Capabilities{
			MutatesData: false,
		}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown))
}
