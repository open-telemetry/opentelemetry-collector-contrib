// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// standardFailoverFactory creates standard failover strategies
type standardFailoverFactory struct{}

func (f *standardFailoverFactory) CreateTracesStrategy(router *baseFailoverRouter[consumer.Traces]) TracesFailoverStrategy {
	cfg := router.cfg
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
	}
	selector := state.NewPipelineSelector(notifyRetry, done, pSConstants)
	errTryLock := state.NewTryLock()
	strategy := &standardStrategy[consumer.Traces]{
		router:      router,
		done:        done,
		notifyRetry: notifyRetry,
		pS:          selector,
		errTryLock:  errTryLock,
	}
	return &standardTracesStrategy{strategy}
}

func (f *standardFailoverFactory) CreateMetricsStrategy(router *baseFailoverRouter[consumer.Metrics]) MetricsFailoverStrategy {
	cfg := router.cfg
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
	}
	selector := state.NewPipelineSelector(notifyRetry, done, pSConstants)
	errTryLock := state.NewTryLock()
	strategy := &standardStrategy[consumer.Metrics]{
		router:      router,
		done:        done,
		notifyRetry: notifyRetry,
		pS:          selector,
		errTryLock:  errTryLock,
	}
	return &standardMetricsStrategy{strategy}
}

func (f *standardFailoverFactory) CreateLogsStrategy(router *baseFailoverRouter[consumer.Logs]) LogsFailoverStrategy {
	cfg := router.cfg
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
	}
	selector := state.NewPipelineSelector(notifyRetry, done, pSConstants)
	errTryLock := state.NewTryLock()
	strategy := &standardStrategy[consumer.Logs]{
		router:      router,
		done:        done,
		notifyRetry: notifyRetry,
		pS:          selector,
		errTryLock:  errTryLock,
	}
	return &standardLogsStrategy{strategy}
}

type standardStrategy[C any] struct {
	router *baseFailoverRouter[C]

	pS          *state.PipelineSelector
	errTryLock  *state.TryLock
	notifyRetry chan struct{}
	done        chan struct{}
}

// getNotifyRetryChannel returns the retry notification channel for strategies to use
func (s *standardStrategy[C]) getNotifyRetryChannel() <-chan struct{} {
	return s.notifyRetry
}

// getCurrentConsumer returns the consumer for the current healthy level
func (s *standardStrategy[C]) getCurrentConsumer() (C, int) {
	var nilConsumer C
	pl := s.pS.CurrentPipeline()
	if pl >= len(s.router.cfg.PipelinePriority) {
		return nilConsumer, pl
	}
	return s.router.consumers[pl], pl
}

// reportConsumerError ensures only one consumer is reporting an error at a time to avoid multiple failovers
func (s *standardStrategy[C]) reportConsumerError(idx int) {
	//fmt.Println("Calling reportConsumerError")
	s.errTryLock.TryExecute(s.pS.HandleError, idx)
}

func (s *standardStrategy[C]) Shutdown() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

// For Testing

func (s *standardStrategy[C]) TestGetCurrentConsumerIndex() int {
	return s.pS.CurrentPipeline()
}

func (s *standardStrategy[C]) TestSetStableConsumerIndex(idx int) {
	s.pS.TestSetCurrentPipeline(idx)
}

//// standardTracesStrategy implements the current standard failover logic for traces
//type standardTracesStrategy struct {
//	router *baseFailoverRouter[consumer.Traces]
//}

type standardTracesStrategy struct {
	*standardStrategy[consumer.Traces]
}

func (s *standardTracesStrategy) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	//fmt.Println("Calling ConsumeTraces")
	select {
	case <-s.getNotifyRetryChannel():
		if !s.sampleRetryConsumers(ctx, td) {
			return s.consumeByHealthyPipeline(ctx, td)
		}
		return nil
	default:
		return s.consumeByHealthyPipeline(ctx, td)
	}
}

func (s *standardTracesStrategy) consumeByHealthyPipeline(ctx context.Context, td ptrace.Traces) error {
	//fmt.Println("Calling consumeByHealthyPipeline")
	for {
		tc, idx := s.getCurrentConsumer()
		if idx >= len(s.router.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeTraces(ctx, td); err != nil {
			s.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

func (s *standardTracesStrategy) sampleRetryConsumers(ctx context.Context, td ptrace.Traces) bool {
	//fmt.Println("Calling sampleRetryConsumers")
	stableIndex := s.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := s.router.getConsumerAtIndex(i)
		err := consumer.ConsumeTraces(ctx, td)
		if err == nil {
			s.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

func (s *standardTracesStrategy) Shutdown() {
	s.standardStrategy.Shutdown()
}

//// standardMetricsStrategy implements the current standard failover logic for metrics
//type standardMetricsStrategy struct {
//	router *baseFailoverRouter[consumer.Metrics]
//}

type standardMetricsStrategy struct {
	*standardStrategy[consumer.Metrics]
}

func (s *standardMetricsStrategy) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	select {
	case <-s.getNotifyRetryChannel():
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
		tc, idx := s.getCurrentConsumer()
		if idx >= len(s.router.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeMetrics(ctx, md); err != nil {
			s.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

func (s *standardMetricsStrategy) sampleRetryConsumers(ctx context.Context, md pmetric.Metrics) bool {
	stableIndex := s.pS.CurrentPipeline()
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
	s.standardStrategy.Shutdown()
}

//// standardLogsStrategy implements the current standard failover logic for logs
//type standardLogsStrategy struct {
//	router *baseFailoverRouter[consumer.Logs]
//}

type standardLogsStrategy struct {
	*standardStrategy[consumer.Logs]
}

func (s *standardLogsStrategy) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	select {
	case <-s.getNotifyRetryChannel():
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
		tc, idx := s.getCurrentConsumer()
		if idx >= len(s.router.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeLogs(ctx, ld); err != nil {
			s.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

func (s *standardLogsStrategy) sampleRetryConsumers(ctx context.Context, ld plog.Logs) bool {
	stableIndex := s.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := s.router.getConsumerAtIndex(i)
		err := consumer.ConsumeLogs(ctx, ld)
		if err == nil {
			s.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

func (s *standardLogsStrategy) Shutdown() {
	s.standardStrategy.Shutdown()
}
