// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"errors"

	"go.opentelemetry.io/collector/pipeline"

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

func (f *baseFailoverRouter[C]) Shutdown() {
	close(f.done)
}

func newBaseFailoverRouter[C any](provider consumerProvider[C], cfg *Config) (*baseFailoverRouter[C], error) {
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
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
	return &baseFailoverRouter[C]{
		consumers:   consumers,
		cfg:         cfg,
		pS:          selector,
		errTryLock:  state.NewTryLock(),
		done:        done,
		notifyRetry: notifyRetry,
	}, nil
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
