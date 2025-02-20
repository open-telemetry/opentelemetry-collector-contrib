// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
	errInvalidConsumer = errors.New("Invalid consumer type")
)

type consumerProvider[C any] func(...pipeline.ID) (C, error)

// telemetryType is a constraint that ties together telemetry data types
type telemetryType interface {
	plog.Logs | pmetric.Metrics | ptrace.Traces
}

// failoverRouter handles the routing of telemetry data to the appropriate consumer
type failoverRouter[T telemetryType, C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
	pS               *state.PipelineSelector
	consumers        []C

	errTryLock  *state.TryLock
	notifyRetry chan struct{}
	done        chan struct{}
}

// Consume is the generic consumption method for all signals
func (f *failoverRouter[T, C]) Consume(ctx context.Context, pd T) error {
	select {
	case <-f.notifyRetry:
		if !f.sampleRetryConsumers(ctx, pd) {
			return f.consumeByHealthyPipeline(ctx, pd)
		}
		return nil
	default:
		return f.consumeByHealthyPipeline(ctx, pd)
	}
}

// consumeByHealthyPipeline will consume the pdata by the current healthy level
func (f *failoverRouter[T, C]) consumeByHealthyPipeline(ctx context.Context, pd T) error {
	for {
		tc, idx := f.getCurrentConsumer()
		if idx >= len(f.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		err := f.consume(ctx, tc, pd)
		if err != nil {
			f.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

// getCurrentConsumer returns the consumer for the current healthy level
func (f *failoverRouter[T, C]) getCurrentConsumer() (C, int) {
	var nilConsumer C
	pl := f.pS.CurrentPipeline()
	if pl >= len(f.cfg.PipelinePriority) {
		return nilConsumer, pl
	}
	return f.consumers[pl], pl
}

// getConsumerAtIndex returns the consumer at a specific index
func (f *failoverRouter[T, C]) getConsumerAtIndex(idx int) C {
	return f.consumers[idx]
}

// sampleRetryConsumers iterates through all unhealthy consumers to re-establish a healthy connection
func (f *failoverRouter[T, C]) sampleRetryConsumers(ctx context.Context, pd T) bool {
	stableIndex := f.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := f.getConsumerAtIndex(i)
		err := f.consume(ctx, consumer, pd)
		if err == nil {
			f.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

// consume routes the telemetry data to the appropriate consumer based on type
func (f *failoverRouter[T, C]) consume(ctx context.Context, c C, pd T) error {
	switch data := any(pd).(type) {
	case plog.Logs:
		if consumer, ok := any(c).(consumer.Logs); ok {
			return consumer.ConsumeLogs(ctx, data)
		}
	case pmetric.Metrics:
		if consumer, ok := any(c).(consumer.Metrics); ok {
			return consumer.ConsumeMetrics(ctx, data)
		}
	case ptrace.Traces:
		if consumer, ok := any(c).(consumer.Traces); ok {
			return consumer.ConsumeTraces(ctx, data)
		}
	}
	return fmt.Errorf("%w for type %T", errInvalidConsumer, pd)
}

// reportConsumerError ensures only one consumer is reporting an error at a time to avoid multiple failovers
func (f *failoverRouter[T, C]) reportConsumerError(idx int) {
	f.errTryLock.TryExecute(f.pS.HandleError, idx)
}

func (f *failoverRouter[T, C]) Shutdown() {
	close(f.done)
}

// registersConsumers registers all consumers
func (f *failoverRouter[T, C]) registerConsumers() error {
	consumers := make([]C, 0)
	for _, pipelines := range f.cfg.PipelinePriority {
		baseConsumer, err := f.consumerProvider(pipelines...)
		if err != nil {
			return errConsumer
		}
		consumers = append(consumers, baseConsumer)
	}
	f.consumers = consumers
	return nil
}

func newFailoverRouter[T telemetryType, C any](provider consumerProvider[C], cfg *Config) *failoverRouter[T, C] {
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
	}

	selector := state.NewPipelineSelector(notifyRetry, done, pSConstants)
	return &failoverRouter[T, C]{
		consumerProvider: provider,
		cfg:              cfg,
		pS:               selector,
		errTryLock:       state.NewTryLock(),
		done:             done,
		notifyRetry:      notifyRetry,
	}
}

// For Testing
func (f *failoverRouter[T, C]) ModifyConsumerAtIndex(idx int, c C) {
	f.consumers[idx] = c
}

func (f *failoverRouter[T, C]) TestGetCurrentConsumerIndex() int {
	return f.pS.CurrentPipeline()
}

func (f *failoverRouter[T, C]) TestSetStableConsumerIndex(idx int) {
	f.pS.TestSetCurrentPipeline(idx)
}

func (f *failoverRouter[T, C]) TestGetConsumerAtIndex(idx int) C {
	return f.consumers[idx]
}
