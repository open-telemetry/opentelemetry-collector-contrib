// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
)

type consumerProvider[C any] func(...component.ID) (C, error)

type failoverRouter[C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
	pS               *pipelineSelector
	consumers        []C
	cancelRetry      *cancelManager
}

// Manages cancel function for retry goroutine, ends up cleaner than using channels
type cancelManager struct {
	lock        sync.Mutex
	cancelRetry context.CancelFunc
}

func (m *cancelManager) updateCancelFunc(newCancelFunc context.CancelFunc) {
	m.lock.Lock()
	m.cancelRetry = newCancelFunc
	m.lock.Unlock()
}

func (m *cancelManager) invokeCancel() {
	m.lock.Lock()
	if m.cancelRetry != nil {
		m.cancelRetry()
	}
	m.lock.Unlock()
}

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
)

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {
	return &failoverRouter[C]{
		consumerProvider: provider,
		cfg:              cfg,
		pS:               newPipelineSelector(cfg),
		cancelRetry:      &cancelManager{},
	}
}

func (f *failoverRouter[C]) getCurrentConsumer() C {
	return f.consumers[f.pS.currentIndex]
}

func (f *failoverRouter[C]) registerConsumers() error {
	consumers := make([]C, 0)
	for _, pipeline := range f.cfg.PipelinePriority {
		newConsumer, err := f.consumerProvider(pipeline...)
		if err == nil {
			consumers = append(consumers, newConsumer)
		} else {
			return errConsumer
		}
	}
	f.consumers = consumers
	return nil
}

// handlePipelineError evaluates if the error was due to a failed retry or a new failure and will resolve the error
// by adjusting the priority level accordingly
func (f *failoverRouter[C]) handlePipelineError() {
	if f.pS.handleErrorRetryCheck() {
		ctx, cancel := context.WithCancel(context.Background())
		f.cancelRetry.invokeCancel()
		f.cancelRetry.updateCancelFunc(cancel)
		f.enableRetry(ctx)
	}
}

func (f *failoverRouter[C]) enableRetry(ctx context.Context) {

	var cancelFunc context.CancelFunc
	var tickerCtx context.Context
	ticker := time.NewTicker(f.cfg.RetryInterval)

	go func() {
		for f.checkContinueRetry(f.pS.stableIndex) {
			select {
			case <-ticker.C:
				if cancelFunc != nil {
					cancelFunc()
				}
				tickerCtx, cancelFunc = context.WithCancel(ctx)
				go f.retryHighPriorityPipelines(tickerCtx, f.pS.stableIndex)
			case <-ctx.Done():
				cancelFunc()
				return
			}
		}
	}()
}

func (f *failoverRouter[C]) pipelineIsValid() bool {
	return f.pS.currentIndex < len(f.cfg.PipelinePriority)
}

func (f *failoverRouter[C]) retryHighPriorityPipelines(ctx context.Context, stableIndex int) {
	ticker := time.NewTicker(f.cfg.RetryGap)

	defer ticker.Stop()

	for i := 0; i < stableIndex; i++ {
		if stableIndex > f.pS.getStableIndex() {
			return
		}
		if f.pS.maxRetriesUsed(i) {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.pS.setToRetryIndex(i)
		}
	}
}

// checkStopRetry checks if retry should be suspended if all higher priority levels have exceeded their max retries
func (f *failoverRouter[C]) checkContinueRetry(index int) bool {
	for i := 0; i < index; i++ {
		if f.pS.pipelineRetries[i] < f.cfg.MaxRetries {
			return true
		}
	}
	return false
}

// reportStable reports back to the failoverRouter that the current priority level that was called by Consume.SIGNAL was
// stable
func (f *failoverRouter[C]) reportStable() {
	if f.pS.currentIndexIsStable() {
		return
	}
	f.pS.setStable()
}

func (f *failoverRouter[C]) handleShutdown() {
	f.cancelRetry.invokeCancel() // maybe change back to channel close
}

// pipelineSelector is meant to serve as the source of truth for the target priority level
type pipelineSelector struct {
	currentIndex    int
	stableIndex     int
	lock            sync.RWMutex
	pipelineRetries []int
	maxRetry        int
}

func (p *pipelineSelector) handleErrorRetryCheck() bool {
	if p.currentIndexIsStable() {
		p.nextPipeline()
		return true
	}
	p.setToStableIndex()
	return false
}

func (p *pipelineSelector) nextPipeline() {
	p.lock.Lock()
	for ok := true; ok; ok = p.pipelineRetries[p.currentIndex] >= p.maxRetry {
		p.currentIndex++
	}
	p.stableIndex = p.currentIndex
	p.lock.Unlock()
}

func (p *pipelineSelector) setToStableIndex() {
	p.lock.Lock()
	p.pipelineRetries[p.currentIndex]++
	p.currentIndex = p.stableIndex
	p.lock.Unlock()
}

func (p *pipelineSelector) setToRetryIndex(index int) {
	p.lock.Lock()
	p.currentIndex = index
	p.lock.Unlock()
}

func (p *pipelineSelector) maxRetriesUsed(index int) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.pipelineRetries[index] >= p.maxRetry
}

func (p *pipelineSelector) setStable() {
	p.lock.Lock()
	p.pipelineRetries[p.currentIndex] = 0
	p.stableIndex = p.currentIndex
	p.lock.Unlock()
}

func (p *pipelineSelector) currentIndexIsStable() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.currentIndex == p.stableIndex
}

func (p *pipelineSelector) getStableIndex() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.stableIndex
}

func newPipelineSelector(cfg *Config) *pipelineSelector {
	return &pipelineSelector{
		currentIndex:    0,
		stableIndex:     0,
		lock:            sync.RWMutex{},
		pipelineRetries: make([]int, len(cfg.PipelinePriority)),
		maxRetry:        cfg.MaxRetries,
	}
}
