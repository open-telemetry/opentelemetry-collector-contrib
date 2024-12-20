// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// PipelineSelector is meant to serve as the source of truth for the target priority level
type PipelineSelector struct {
	currentIndex    atomic.Int32
	stableIndex     atomic.Int32
	pipelineRetries []atomic.Int32
	constants       PSConstants
	RS              *RetryState

	errTryLock    *TryLock
	stableTryLock *TryLock
	chans         []chan bool
}

func (p *PipelineSelector) handlePipelineError(idx int) {
	if idx != p.loadCurrent() {
		return
	}
	doRetry := p.indexIsStable(idx)
	p.updatePipelineIndex(idx)
	if !doRetry {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.RS.InvokeCancel()
	p.RS.UpdateCancelFunc(cancel)
	p.enableRetry(ctx, p.constants.RetryInterval, p.constants.RetryGap)
}

func (p *PipelineSelector) enableRetry(ctx context.Context, retryInterval time.Duration, retryGap time.Duration) {
	go func() {
		ticker := time.NewTicker(retryInterval)
		defer ticker.Stop()

		var cancelFunc context.CancelFunc
		for p.checkContinueRetry(p.loadStable()) {
			select {
			case <-ticker.C:
				if cancelFunc != nil {
					cancelFunc()
				}
				cancelFunc = p.handleRetry(ctx, retryGap)
			case <-ctx.Done():
				return
			}
		}
		p.RS.InvokeCancel()
	}()
}

// handleRetry is responsible for launching goroutine and returning cancelFunc
func (p *PipelineSelector) handleRetry(parentCtx context.Context, retryGap time.Duration) context.CancelFunc {
	retryCtx, cancelFunc := context.WithCancel(parentCtx)
	go p.retryHighPriorityPipelines(retryCtx, retryGap)
	return cancelFunc
}

// UpdatePipelineIndex is the main function that updates the pipeline indexes due to an error
func (p *PipelineSelector) updatePipelineIndex(idx int) {
	if p.indexIsStable(idx) {
		p.setToNextPriorityPipeline(idx)
		return
	}
	p.setToStableIndex(idx)
}

// NextPipeline skips through any lower priority pipelines that have exceeded their maxRetries
func (p *PipelineSelector) setToNextPriorityPipeline(idx int) {
	for ok := true; ok; ok = p.exceededMaxRetries(idx) {
		idx++
	}
	p.stableIndex.Store(int32(idx))
	p.currentIndex.Store(int32(idx))
}

// RetryHighPriorityPipelines responsible for single iteration through all higher priority pipelines
func (p *PipelineSelector) retryHighPriorityPipelines(ctx context.Context, retryGap time.Duration) {
	ticker := time.NewTicker(retryGap)

	defer ticker.Stop()

	for i := 0; i < len(p.pipelineRetries); i++ {
		if p.maxRetriesUsed(i) {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if i >= p.loadStable() {
				return
			}
			p.currentIndex.Store(int32(i))
		}
	}
}

// checkContinueRetry checks if retry should be suspended if all higher priority levels have exceeded their max retries
func (p *PipelineSelector) checkContinueRetry(index int) bool {
	for i := 0; i < index; i++ {
		if p.loadRetryCount(i) < p.constants.MaxRetries {
			return true
		}
	}
	return false
}

func (p *PipelineSelector) exceededMaxRetries(idx int) bool {
	return p.constants.MaxRetries > 0 && idx < len(p.pipelineRetries) && (p.loadRetryCount(idx) >= p.constants.MaxRetries)
}

// SetToStableIndex returns the CurrentIndex to the known Stable Index
func (p *PipelineSelector) setToStableIndex(idx int) {
	p.incrementRetryCount(idx)
	p.currentIndex.Store(p.stableIndex.Load())
}

// MaxRetriesUsed exported access to maxRetriesUsed
func (p *PipelineSelector) maxRetriesUsed(idx int) bool {
	return p.loadRetryCount(idx) >= p.constants.MaxRetries
}

// SetNewStableIndex Update stableIndex to the passed stable index
func (p *PipelineSelector) setNewStableIndex(idx int) {
	p.resetRetryCount(idx)
	p.stableIndex.Store(int32(idx))
}

// IndexIsStable returns if index passed is the stable index
func (p *PipelineSelector) indexIsStable(idx int) bool {
	return p.loadStable() == idx
}

func (p *PipelineSelector) loadStable() int {
	return int(p.stableIndex.Load())
}

func (p *PipelineSelector) loadCurrent() int {
	return int(p.currentIndex.Load())
}

func (p *PipelineSelector) loadRetryCount(idx int) int {
	return int(p.pipelineRetries[idx].Load())
}

func (p *PipelineSelector) incrementRetryCount(idx int) {
	p.pipelineRetries[idx].Add(1)
}

func (p *PipelineSelector) resetRetryCount(idx int) {
	p.pipelineRetries[idx].Store(0)
}

// ReportStable reports back to the failoverRouter that the current priority was stable
func (p *PipelineSelector) reportStable(idx int) {
	if p.indexIsStable(idx) {
		return
	}
	p.setNewStableIndex(idx)
}

func NewPipelineSelector(lenPriority int, consts PSConstants) *PipelineSelector {
	chans := make([]chan bool, lenPriority)

	for i := 0; i < lenPriority; i++ {
		chans[i] = make(chan bool)
	}

	ps := &PipelineSelector{
		pipelineRetries: make([]atomic.Int32, lenPriority),
		constants:       consts,
		RS:              &RetryState{},
		errTryLock:      NewTryLock(),
		stableTryLock:   NewTryLock(),
		chans:           chans,
	}
	return ps
}

func (p *PipelineSelector) Start(done chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	go p.ListenToChannels(done, wg)
}

func (p *PipelineSelector) ListenToChannels(done chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	cases := make([]reflect.SelectCase, len(p.chans)+1)
	for i, ch := range p.chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	cases[len(p.chans)] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(done)}

	for {
		chosen, value, ok := reflect.Select(cases)
		if !ok {
			return
		}
		if value.Bool() {
			p.stableTryLock.TryExecute(p.reportStable, chosen)
		} else {
			p.errTryLock.TryExecute(p.handlePipelineError, chosen)
		}
	}
}

func (p *PipelineSelector) SelectedPipeline() (int, chan bool) {
	idx := p.loadCurrent()
	if idx < len(p.chans) {
		return idx, p.chans[idx]
	}
	return idx, nil
}

// For Testing
func (p *PipelineSelector) ChannelIndex(ch chan bool) int {
	for i, ch1 := range p.chans {
		if ch == ch1 {
			return i
		}
	}
	return -1
}

func (p *PipelineSelector) TestStableIndex() int {
	return p.loadStable()
}

func (p *PipelineSelector) TestCurrentIndex() int {
	return p.loadCurrent()
}

func (p *PipelineSelector) TestSetStableIndex(idx int32) {
	p.stableIndex.Store(idx)
}

func (p *PipelineSelector) TestRetryPipelines(ctx context.Context, retryInterval time.Duration, retryGap time.Duration) {
	p.enableRetry(ctx, retryInterval, retryGap)
}

func (p *PipelineSelector) SetRetryCountToMax(idx int) {
	p.pipelineRetries[idx].Store(int32(p.constants.MaxRetries))
}

func (p *PipelineSelector) ResetRetryCount(idx int) {
	p.pipelineRetries[idx].Store(0)
}
