// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// progressiveFailoverFactory creates progressive failover strategies
type progressiveFailoverFactory struct{}

func (f *progressiveFailoverFactory) CreateTracesStrategy(router *baseFailoverRouter[consumer.Traces]) TracesFailoverStrategy {
	return &progressiveTracesStrategy{
		router: router,
	}
}

func (f *progressiveFailoverFactory) CreateMetricsStrategy(router *baseFailoverRouter[consumer.Metrics]) MetricsFailoverStrategy {
	return &progressiveMetricsStrategy{
		router: router,
	}
}

func (f *progressiveFailoverFactory) CreateLogsStrategy(router *baseFailoverRouter[consumer.Logs]) LogsFailoverStrategy {
	return &progressiveLogsStrategy{
		router: router,
	}
}

type progressiveTracesStrategy struct {
	router *baseFailoverRouter[consumer.Traces]
}

func (p *progressiveTracesStrategy) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < len(p.router.cfg.PipelinePriority); i++ {
		consumer := p.router.getConsumerAtIndex(i)
		if err := consumer.ConsumeTraces(ctx, td); err != nil {
			continue
		}
		return nil
	}
	return errNoValidPipeline
}

func (p *progressiveTracesStrategy) Shutdown() {
}

type progressiveMetricsStrategy struct {
	router *baseFailoverRouter[consumer.Metrics]
}

func (p *progressiveMetricsStrategy) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for i := 0; i < len(p.router.cfg.PipelinePriority); i++ {
		consumer := p.router.getConsumerAtIndex(i)
		if err := consumer.ConsumeMetrics(ctx, md); err != nil {
			continue
		}
		return nil
	}
	return errNoValidPipeline
}

func (p *progressiveMetricsStrategy) Shutdown() {
}

type progressiveLogsStrategy struct {
	router *baseFailoverRouter[consumer.Logs]
}

func (p *progressiveLogsStrategy) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for i := 0; i < len(p.router.cfg.PipelinePriority); i++ {
		consumer := p.router.getConsumerAtIndex(i)
		if err := consumer.ConsumeLogs(ctx, ld); err != nil {
			continue
		}
		return nil
	}
	return errNoValidPipeline
}

func (p *progressiveLogsStrategy) Shutdown() {
}
