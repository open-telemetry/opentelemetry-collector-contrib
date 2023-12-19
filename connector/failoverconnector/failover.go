// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

type consumerProvider[C any] func(...component.ID) (C, error)

type failoverRouter[C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
	pS               *state.PipelineSelector
	consumers        []C
	rS               *state.RetryState
}

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
)

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {
	return &failoverRouter[C]{
		consumerProvider: provider,
		cfg:              cfg,
		pS:               state.NewPipelineSelector(len(cfg.PipelinePriority), cfg.MaxRetries),
		rS:               &state.RetryState{},
	}
}

func (f *failoverRouter[C]) getCurrentConsumer() (C, int, bool) {
	// if currentIndex incremented passed bounds of pipeline list
	var nilConsumer C
	idx := f.pS.CurrentIndex()
	if idx >= len(f.cfg.PipelinePriority) {
		return nilConsumer, -1, false
	}
	return f.consumers[idx], idx, true
}

func (f *failoverRouter[C]) registerConsumers() error {
	consumers := make([]C, 0)
	for _, pipelines := range f.cfg.PipelinePriority {
		newConsumer, err := f.consumerProvider(pipelines...)
		if err != nil {
			return errConsumer
		}
		consumers = append(consumers, newConsumer)
	}
	f.consumers = consumers
	return nil
}

func (f *failoverRouter[C]) handlePipelineError(idx int) {
	// avoids race condition in case of consumeSIGNAL invocations
	// where index was updated during execution
	if idx != f.pS.CurrentIndex() {
		return
	}
	doRetry := f.pS.IndexIsStable(idx)
	// UpdatePipelineIndex either increments the pipeline to the next priority
	// or returns it to the stable
	f.pS.UpdatePipelineIndex(idx)
	// if the currentIndex is not the stableIndex, that means the currentIndex is a higher
	// priority index that was set during a retry, in which case we don't want to start a
	// new retry goroutine
	if !doRetry {
		return
	}
	// kill existing retry goroutine if error is from a stable pipeline that failed for the first time
	ctx, cancel := context.WithCancel(context.Background())
	f.rS.InvokeCancel()
	f.rS.UpdateCancelFunc(cancel)
	f.enableRetry(ctx)
}

func (f *failoverRouter[C]) enableRetry(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(f.cfg.RetryInterval)
		defer ticker.Stop()

		stableIndex := f.pS.StableIndex()
		var cancelFunc context.CancelFunc
		// checkContinueRetry checks that any higher priority levels have retries remaining
		// (have not exceeded their maxRetries)
		for f.checkContinueRetry(stableIndex) {
			select {
			case <-ticker.C:
				// When the nextRetry interval starts we kill the existing iteration through
				// the higher priority pipelines if still in progress
				if cancelFunc != nil {
					cancelFunc()
				}
				cancelFunc = f.handleRetry(ctx, stableIndex)
			case <-ctx.Done():
				return
			}
		}
		f.rS.InvokeCancel()
	}()
}

// handleRetry is responsible for launching goroutine and returning cancelFunc for context to be called if new
// interval starts in the middle of the execution
func (f *failoverRouter[C]) handleRetry(parentCtx context.Context, stableIndex int) context.CancelFunc {
	retryCtx, cancelFunc := context.WithCancel(parentCtx)
	go f.pS.RetryHighPriorityPipelines(retryCtx, stableIndex, f.cfg.RetryGap)
	return cancelFunc
}

// checkStopRetry checks if retry should be suspended if all higher priority levels have exceeded their max retries
func (f *failoverRouter[C]) checkContinueRetry(index int) bool {
	for i := 0; i < index; i++ {
		if f.pS.IndexRetryCount(i) < f.cfg.MaxRetries {
			return true
		}
	}
	return false
}

// reportStable reports back to the failoverRouter that the current priority level that was called by Consume.SIGNAL was
// stable
func (f *failoverRouter[C]) reportStable(idx int) {
	f.pS.ReportStable(idx)
}

// For Testing
func (f *failoverRouter[C]) GetConsumerAtIndex(idx int) C {
	return f.consumers[idx]
}

func (f *failoverRouter[C]) ModifyConsumerAtIndex(idx int, c C) {
	f.consumers[idx] = c
}
