// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

// standardFailoverFactory creates standard failover strategies
type standardFailoverFactory struct{}

func (*standardFailoverFactory) CreateTracesStrategy(router *baseFailoverRouter[consumer.Traces]) tracesFailoverStrategy {
	return &standardTracesStrategy{newStandardStrategy(router)}
}

func (*standardFailoverFactory) CreateMetricsStrategy(router *baseFailoverRouter[consumer.Metrics]) metricsFailoverStrategy {
	return &standardMetricsStrategy{newStandardStrategy(router)}
}

func (*standardFailoverFactory) CreateLogsStrategy(router *baseFailoverRouter[consumer.Logs]) logsFailoverStrategy {
	return &standardLogsStrategy{newStandardStrategy(router)}
}

type standardStrategy[C any] struct {
	router *baseFailoverRouter[C]

	selector     *state.PipelineSelector
	errTryLock   *state.TryLock
	notifyRetry  chan struct{}
	done         chan struct{}
	shutdownOnce sync.Once
}

func newStandardStrategy[C any](router *baseFailoverRouter[C]) *standardStrategy[C] {
	cfg := router.cfg
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.effectiveRetryInterval(),
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
	}
	return &standardStrategy[C]{
		router:      router,
		done:        done,
		notifyRetry: notifyRetry,
		selector:    state.NewPipelineSelector(notifyRetry, done, pSConstants),
		errTryLock:  state.NewTryLock(),
	}
}

// getCurrentConsumer returns the consumer for the current healthy level
func (s *standardStrategy[C]) getCurrentConsumer() (C, int) {
	var nilConsumer C
	pl := s.selector.CurrentPipeline()
	if pl >= len(s.router.cfg.PipelinePriority) {
		return nilConsumer, pl
	}
	return s.router.consumers[pl], pl
}

// reportConsumerError ensures only one consumer is reporting an error at a time to avoid multiple failovers
func (s *standardStrategy[C]) reportConsumerError(idx int) {
	s.errTryLock.TryExecute(s.selector.HandleError, idx)
}

func (s *standardStrategy[C]) Shutdown() {
	s.shutdownOnce.Do(func() { close(s.done) })
}

// For Testing

func (s *standardStrategy[C]) TestGetCurrentConsumerIndex() int {
	return s.selector.CurrentPipeline()
}

func (s *standardStrategy[C]) TestSetStableConsumerIndex(idx int) {
	s.selector.TestSetCurrentPipeline(idx)
}

func consume[C, D any](ctx context.Context, s *standardStrategy[C], data D, fn func(C, context.Context, D) error) error {
	select {
	case <-s.notifyRetry:
		if !sampleRetryConsumers(ctx, s, data, fn) {
			return consumeByHealthyPipeline(ctx, s, data, fn)
		}
		return nil
	default:
		return consumeByHealthyPipeline(ctx, s, data, fn)
	}
}

func consumeByHealthyPipeline[C, D any](ctx context.Context, s *standardStrategy[C], data D, fn func(C, context.Context, D) error) error {
	for {
		c, idx := s.getCurrentConsumer()
		if idx >= len(s.router.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := fn(c, ctx, data); err != nil {
			s.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

func sampleRetryConsumers[C, D any](ctx context.Context, s *standardStrategy[C], data D, fn func(C, context.Context, D) error) bool {
	stableIndex := s.selector.CurrentPipeline()
	for i := range stableIndex {
		c := s.router.getConsumerAtIndex(i)
		if err := fn(c, ctx, data); err == nil {
			s.selector.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

type standardTracesStrategy struct {
	*standardStrategy[consumer.Traces]
}

func (s *standardTracesStrategy) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return consume(ctx, s.standardStrategy, td, consumer.Traces.ConsumeTraces)
}

type standardMetricsStrategy struct {
	*standardStrategy[consumer.Metrics]
}

func (s *standardMetricsStrategy) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return consume(ctx, s.standardStrategy, md, consumer.Metrics.ConsumeMetrics)
}

type standardLogsStrategy struct {
	*standardStrategy[consumer.Logs]
}

func (s *standardLogsStrategy) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return consume(ctx, s.standardStrategy, ld, consumer.Logs.ConsumeLogs)
}
