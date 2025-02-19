// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
)

type consumerProvider[C any] func(...pipeline.ID) (C, error)

type wrapConsumer[C any] func(consumer C) internal.SignalConsumer

type failoverRouter[C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
	pS               *state.PipelineSelector
	consumers        []internal.SignalConsumer

	errTryLock  *state.TryLock
	notifyRetry chan struct{}
	done        chan struct{}
}

// Consume is the generic consumption method for all signals
func (f *failoverRouter[C]) Consume(ctx context.Context, pd any) error {
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
func (f *failoverRouter[C]) consumeByHealthyPipeline(ctx context.Context, pd any) error {
	for {
		tc, idx := f.getCurrentConsumer()
		if idx >= len(f.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.Consume(ctx, pd); err != nil {
			f.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

// getCurrentConsumer returns the consumer for the current healthy level
func (f *failoverRouter[C]) getCurrentConsumer() (internal.SignalConsumer, int) {
	var nilConsumer internal.SignalConsumer
	pl := f.pS.CurrentPipeline()
	if pl >= len(f.cfg.PipelinePriority) {
		return nilConsumer, pl
	}
	return f.consumers[pl], pl
}

// getConsumerAtIndex returns the consumer at a specific index
func (f *failoverRouter[C]) getConsumerAtIndex(idx int) internal.SignalConsumer {
	return f.consumers[idx]
}

// sampleRetryConsumers iterates through all unhealthy consumers to re-establish a healthy connection
func (f *failoverRouter[C]) sampleRetryConsumers(ctx context.Context, pd any) bool {
	stableIndex := f.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := f.getConsumerAtIndex(i)
		err := consumer.Consume(ctx, pd)
		if err == nil {
			f.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

// reportConsumerError ensures only one consumer is reporting an error at a time to avoid multiple failovers
func (f *failoverRouter[C]) reportConsumerError(idx int) {
	f.errTryLock.TryExecute(f.pS.HandleError, idx)
}

func (f *failoverRouter[C]) Shutdown() {
	close(f.done)
}

// registersConsumers registers all consumers and converts all to internal.SignalConsumer
func (f *failoverRouter[C]) registerConsumers(wrap wrapConsumer[C]) error {
	consumers := make([]internal.SignalConsumer, 0)
	for _, pipelines := range f.cfg.PipelinePriority {
		baseConsumer, err := f.consumerProvider(pipelines...)
		if err != nil {
			return errConsumer
		}
		newConsumer := wrap(baseConsumer)

		consumers = append(consumers, newConsumer)
	}
	f.consumers = consumers
	return nil
}

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
	}

	selector := state.NewPipelineSelector(notifyRetry, done, pSConstants)
	return &failoverRouter[C]{
		consumerProvider: provider,
		cfg:              cfg,
		pS:               selector,
		errTryLock:       state.NewTryLock(),
		done:             done,
		notifyRetry:      notifyRetry,
	}
}

// For Testing
func (f *failoverRouter[C]) ModifyConsumerAtIndex(idx int, consumerWrapper wrapConsumer[C], c C) {
	f.consumers[idx] = consumerWrapper(c)
}

func (f *failoverRouter[C]) TestGetCurrentConsumerIndex() int {
	return f.pS.CurrentPipeline()
}

func (f *failoverRouter[C]) TestSetStableConsumerIndex(idx int) {
	f.pS.TestSetCurrentPipeline(idx)
}

func (f *failoverRouter[C]) TestGetConsumerAtIndex(idx int) internal.SignalConsumer {
	return f.consumers[idx]
}
