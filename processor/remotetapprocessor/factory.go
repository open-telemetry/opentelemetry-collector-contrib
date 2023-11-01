// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor/internal/metadata"
)

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
