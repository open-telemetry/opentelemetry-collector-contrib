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

// standardFailoverFactory creates standard failover strategies
type standardFailoverFactory struct{}

func (f *standardFailoverFactory) CreateTracesStrategy(router *baseFailoverRouter[consumer.Traces]) TracesFailoverStrategy {
	return &standardTracesStrategy{router: router}
}

func (f *standardFailoverFactory) CreateMetricsStrategy(router *baseFailoverRouter[consumer.Metrics]) MetricsFailoverStrategy {
	return &standardMetricsStrategy{router: router}
}

func (f *standardFailoverFactory) CreateLogsStrategy(router *baseFailoverRouter[consumer.Logs]) LogsFailoverStrategy {
	return &standardLogsStrategy{router: router}
}

// standardTracesStrategy implements the current standard failover logic for traces
type standardTracesStrategy struct {
	router *baseFailoverRouter[consumer.Traces]
}

func (s *standardTracesStrategy) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	select {
	case <-s.router.getNotifyRetryChannel():
		if !s.sampleRetryConsumers(ctx, td) {
			return s.consumeByHealthyPipeline(ctx, td)
		}
		return nil
	default:
		return s.consumeByHealthyPipeline(ctx, td)
	}
}

func (s *standardTracesStrategy) consumeByHealthyPipeline(ctx context.Context, td ptrace.Traces) error {
	for {
		tc, idx := s.router.getCurrentConsumer()
		if idx >= len(s.router.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeTraces(ctx, td); err != nil {
			s.router.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

func (s *standardTracesStrategy) sampleRetryConsumers(ctx context.Context, td ptrace.Traces) bool {
	stableIndex := s.router.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := s.router.getConsumerAtIndex(i)
		err := consumer.ConsumeTraces(ctx, td)
		if err == nil {
			s.router.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

func (s *standardTracesStrategy) Shutdown() {
}

// standardMetricsStrategy implements the current standard failover logic for metrics
type standardMetricsStrategy struct {
	router *baseFailoverRouter[consumer.Metrics]
}

func (s *standardMetricsStrategy) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	select {
	case <-s.router.getNotifyRetryChannel():
		if !s.sampleRetryConsumers(ctx, md) {
			return s.consumeByHealthyPipeline(ctx, md)
		}
		return nil
	default:
		return s.consumeByHealthyPipeline(ctx, md)
	}
}

func (s *standardMetricsStrategy) consumeByHealthyPipeline(ctx context.Context, md pmetric.Metrics) error {
	for {
		tc, idx := s.router.getCurrentConsumer()
		if idx >= len(s.router.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeMetrics(ctx, md); err != nil {
			s.router.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

func (s *standardMetricsStrategy) sampleRetryConsumers(ctx context.Context, md pmetric.Metrics) bool {
	stableIndex := s.router.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := s.router.getConsumerAtIndex(i)
		err := consumer.ConsumeMetrics(ctx, md)
		if err == nil {
			s.router.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

func (s *standardMetricsStrategy) Shutdown() {
}

// standardLogsStrategy implements the current standard failover logic for logs
type standardLogsStrategy struct {
	router *baseFailoverRouter[consumer.Logs]
}

func (s *standardLogsStrategy) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	select {
	case <-s.router.getNotifyRetryChannel():
		if !s.sampleRetryConsumers(ctx, ld) {
			return s.consumeByHealthyPipeline(ctx, ld)
		}
		return nil
	default:
		return s.consumeByHealthyPipeline(ctx, ld)
	}
}

func (s *standardLogsStrategy) consumeByHealthyPipeline(ctx context.Context, ld plog.Logs) error {
	for {
		tc, idx := s.router.getCurrentConsumer()
		if idx >= len(s.router.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeLogs(ctx, ld); err != nil {
			s.router.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

func (s *standardLogsStrategy) sampleRetryConsumers(ctx context.Context, ld plog.Logs) bool {
	stableIndex := s.router.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := s.router.getConsumerAtIndex(i)
		err := consumer.ConsumeLogs(ctx, ld)
		if err == nil {
			s.router.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

func (s *standardLogsStrategy) Shutdown() {
}
