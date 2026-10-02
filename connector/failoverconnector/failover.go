// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
)

type consumerProvider[C any] func(...pipeline.ID) (C, error)

// baseFailoverRouter provides the common infrastructure for failover routing
type baseFailoverRouter[C any] struct {
	cfg       *Config
	pS        *state.PipelineSelector
	consumers []C

	errTryLock  *state.TryLock
	notifyRetry chan struct{}
	done        chan struct{}
	conditions  Condition

	telemetryBuilder *metadata.TelemetryBuilder
}

// getCurrentConsumer returns the consumer for the current healthy level
func (f *baseFailoverRouter[C]) getCurrentConsumer() (C, int) {
	var nilConsumer C
	pl := f.pS.CurrentPipeline()
	if pl >= len(f.cfg.PipelinePriority) {
		return nilConsumer, pl
	}
	return f.consumers[pl], pl
}

// getConsumerAtIndex returns the consumer at a specific index
func (f *baseFailoverRouter[C]) getConsumerAtIndex(idx int) C {
	return f.consumers[idx]
}

// reportConsumerError ensures only one consumer is reporting an error at a time to avoid multiple failovers
func (f *baseFailoverRouter[C]) reportConsumerError(idx int) {
	f.errTryLock.TryExecute(f.pS.HandleError, idx)
}

// shouldFailoverOnError goes through the user defined condition
// to check if given error should cause failover
func (f *baseFailoverRouter[C]) shouldFailoverOnError(err error) bool {
	if f.conditions != nil {
		return f.conditions.ShouldFailover(err)
	}
	return true
}

// registerTelemetry registers the callback that reports which priority level currently receives data
func (f *baseFailoverRouter[C]) registerTelemetry(set connector.Settings) error {
	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return err
	}
	f.telemetryBuilder = tb

	attrs := metric.WithAttributes(attribute.String("connector", set.ID.String()))
	return tb.RegisterConnectorFailoverActiveLevelCallback(func(_ context.Context, o metric.Int64Observer) error {
		current := f.pS.CurrentPipeline()
		if current >= len(f.cfg.PipelinePriority) {
			current = -1
		}
		o.Observe(int64(current), attrs)
		return nil
	})
}

func (f *baseFailoverRouter[C]) Shutdown() {
	select {
	case <-f.done:
	default:
		close(f.done)
	}
	f.telemetryBuilder.Shutdown()
}

func newBaseFailoverRouter[C any](provider consumerProvider[C], cfg *Config, set connector.Settings) (*baseFailoverRouter[C], error) {
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
	}

	consumers := make([]C, 0)
	for _, pipelines := range cfg.PipelinePriority {
		baseConsumer, err := provider(pipelines...)
		if err != nil {
			return nil, errConsumer
		}
		consumers = append(consumers, baseConsumer)
	}

	selector := state.NewPipelineSelector(notifyRetry, done, pSConstants)
	f := &baseFailoverRouter[C]{
		consumers:   consumers,
		cfg:         cfg,
		pS:          selector,
		errTryLock:  state.NewTryLock(),
		done:        done,
		notifyRetry: notifyRetry,
		conditions:  buildCondition(cfg.Condition.Get()),
	}
	if err := f.registerTelemetry(set); err != nil {
		return nil, err
	}
	return f, nil
}

// For Testing
func (f *baseFailoverRouter[C]) ModifyConsumerAtIndex(idx int, c C) {
	f.consumers[idx] = c
}

func (f *baseFailoverRouter[C]) TestGetCurrentConsumerIndex() int {
	return f.pS.CurrentPipeline()
}

func (f *baseFailoverRouter[C]) TestSetStableConsumerIndex(idx int) {
	f.pS.TestSetCurrentPipeline(idx)
}

func (f *baseFailoverRouter[C]) TestGetConsumerAtIndex(idx int) C {
	return f.consumers[idx]
}
