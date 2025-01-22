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
	currentIndex atomic.Int32
	stableIndex  atomic.Int32
	RetryTracker *RetryTracker
	constants    PSConstants
	RS           *RetryState

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
	if doRetry {
		p.enableRetry(p.constants.RetryInterval, p.constants.RetryGap)
	}
}

func (p *PipelineSelector) enableRetry(retryInterval time.Duration, retryGap time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	p.RS.CancelAndSet(cancel)

	go func() {
		ticker := time.NewTicker(retryInterval)
		defer ticker.Stop()

		var cancelFunc context.CancelFunc
		for p.RetryTracker.CheckContinueRetry(p.loadStable()) {
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
		p.RS.cancelManager.Cancel()
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
	p.returnToStableIndex(idx)
}

// NextPipeline skips through any lower priority pipelines that have exceeded their maxRetries
func (p *PipelineSelector) setToNextPriorityPipeline(idx int) {
	for ok := true; ok; ok = p.RetryTracker.ExceededMaxRetries(idx) {
		idx++
	}
	p.stableIndex.Store(int32(idx))
	p.currentIndex.Store(int32(idx))
}

// RetryHighPriorityPipelines responsible for single iteration through all higher priority pipelines
func (p *PipelineSelector) retryHighPriorityPipelines(ctx context.Context, retryGap time.Duration) {
	ticker := time.NewTicker(retryGap)

	defer ticker.Stop()
	for i := 0; i < p.constants.PriorityListLen; i++ {
		if p.RetryTracker.ExceededMaxRetries(i) {
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

// SetToStableIndex returns the CurrentIndex to the known Stable Index
func (p *PipelineSelector) returnToStableIndex(idx int) {
	p.RetryTracker.FailedRetry(idx)
	p.currentIndex.Store(p.stableIndex.Load())
}

// SetNewStableIndex Update stableIndex to the passed stable index
func (p *PipelineSelector) setNewStableIndex(idx int) {
	p.RetryTracker.SuccessfullRetry(idx)
	p.stableIndex.Store(int32(idx))
}

// ReportStable reports back to the failoverRouter that the current priority was stable
func (p *PipelineSelector) reportStable(idx int) {
	if p.indexIsStable(idx) {
		return
	}
	p.setNewStableIndex(idx)
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

func (p *PipelineSelector) Start(done chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	go p.ListenToChannels(done, wg)
}

func (p *PipelineSelector) Shutdown() {
	p.RetryTracker.Shutdown()
	p.RS.Cancel()
}

func NewPipelineSelector(consts PSConstants) *PipelineSelector {
	chans := make([]chan bool, consts.PriorityListLen)

	for i := 0; i < consts.PriorityListLen; i++ {
		chans[i] = make(chan bool)
	}

	ps := &PipelineSelector{
		RetryTracker:  NewRetryTracker(consts.PriorityListLen, consts.MaxRetries, consts.RetryBackoff),
		constants:     consts,
		RS:            &RetryState{},
		errTryLock:    NewTryLock(),
		stableTryLock: NewTryLock(),
		chans:         chans,
	}
	return ps
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

func (p *PipelineSelector) TestRetryPipelines(retryInterval time.Duration, retryGap time.Duration) {
	p.enableRetry(retryInterval, retryGap)
}

func (p *PipelineSelector) SetRetryCountToValue(idx int, val int) {
	p.RetryTracker.SetRetryCountToValue(idx, val)
}

func (p *PipelineSelector) ResetRetryCount(idx int) {
	p.RetryTracker.ResetRetryCount(idx)
}
