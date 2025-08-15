// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsprocessor // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const typeStr = "alertsprocessor"

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetrics, component.StabilityLevelAlpha),
		processor.WithLogs(createLogs, component.StabilityLevelAlpha),
		processor.WithTraces(createTraces, component.StabilityLevelAlpha),
	)
}

func createMetrics(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Metrics) (processor.Metrics, error) {
	c := cfg.(*Config)
	if err := c.Validate(); err != nil {
		return nil, err
	}
	p, err := newProcessor(ctx, set, c, next, nil, nil)
	if err != nil {
		return nil, err
	}

	// Wrap to enforce evaluation.timeout since processorhelper no longer exposes WithTimeouts
	process := func(ctx context.Context, md consumerMetricsArg) (consumerMetricsArg, error) {
		to := c.Evaluation.Timeout
		if to <= 0 {
			return p.processMetrics(ctx, md)
		}
		tctx, cancel := context.WithTimeout(ctx, to)
		defer cancel()
		return p.processMetrics(tctx, md)
	}

	return processorhelper.NewMetrics(
		ctx, set, cfg, next, process,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown),
	)
}

// small type alias to keep the closure readable
type consumerMetricsArg = pmetric.Metrics

func createLogs(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Logs) (processor.Logs, error) {
	c := cfg.(*Config)
	if err := c.Validate(); err != nil {
		return nil, err
	}
	p, err := newProcessor(ctx, set, c, nil, next, nil)
	if err != nil {
		return nil, err
	}

	process := func(ctx context.Context, ld consumerLogsArg) (consumerLogsArg, error) {
		to := c.Evaluation.Timeout
		if to <= 0 {
			return p.processLogs(ctx, ld)
		}
		tctx, cancel := context.WithTimeout(ctx, to)
		defer cancel()
		return p.processLogs(tctx, ld)
	}

	return processorhelper.NewLogs(
		ctx, set, cfg, next, process,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown),
	)
}

type consumerLogsArg = plog.Logs

func createTraces(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	c := cfg.(*Config)
	if err := c.Validate(); err != nil {
		return nil, err
	}
	p, err := newProcessor(ctx, set, c, nil, nil, next)
	if err != nil {
		return nil, err
	}

	process := func(ctx context.Context, td consumerTracesArg) (consumerTracesArg, error) {
		to := c.Evaluation.Timeout
		if to <= 0 {
			return p.processTraces(ctx, td)
		}
		tctx, cancel := context.WithTimeout(ctx, to)
		defer cancel()
		return p.processTraces(tctx, td)
	}

	return processorhelper.NewTraces(
		ctx, set, cfg, next, process,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown),
	)
}

type consumerTracesArg = ptrace.Traces
