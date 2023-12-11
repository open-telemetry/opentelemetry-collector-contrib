// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"sync"
	"time"
)

// PipelineSelector is meant to serve as the source of truth for the target priority level
type PipelineSelector struct {
	currentIndex    int
	stableIndex     int
	lock            sync.RWMutex
	pipelineRetries []int
	maxRetry        int
}

// UpdatePipelineIndex is the main function that updates the pipeline indexes due to an error
// if the currentIndex is not the stableIndex, that means the currentIndex is a higher
// priority index that was set during a retry, in which case we return to the stable index
func (p *PipelineSelector) UpdatePipelineIndex(idx int) {
	if p.IndexIsStable(idx) {
		p.setToNextPriorityPipeline(idx)
		return
	}
	p.setToStableIndex(idx)
}

// NextPipeline skips through any lower priority pipelines that have exceeded their maxRetries
// and sets the first that has not as the new stable
func (p *PipelineSelector) setToNextPriorityPipeline(idx int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	for ok := true; ok; ok = p.exceededMaxRetries(idx) {
		idx++
	}
	p.stableIndex = idx
}

// retryHighPriorityPipelines responsible for single iteration through all higher priority pipelines
func (p *PipelineSelector) RetryHighPriorityPipelines(ctx context.Context, stableIndex int, retryGap time.Duration) {
	ticker := time.NewTicker(retryGap)

	defer ticker.Stop()

	for i := 0; i < stableIndex; i++ {
		// if stableIndex was updated to a higher priority level during the execution of the goroutine
		// will return to avoid overwriting higher priority level with lower one
		if stableIndex > p.StableIndex() {
			return
		}
		// checks that max retries were not used for this index
		if p.MaxRetriesUsed(i) {
			continue
		}
		select {
		// return when context is cancelled by parent goroutine
		case <-ctx.Done():
			return
		case <-ticker.C:
			// when ticker triggers currentIndex is updated
			p.setToCurrentIndex(i)
		}
	}
}

func (p *PipelineSelector) exceededMaxRetries(idx int) bool {
	return idx < len(p.pipelineRetries) && (p.pipelineRetries[idx] >= p.maxRetry)
}

// SetToStableIndex returns the CurrentIndex to the known Stable Index
func (p *PipelineSelector) setToStableIndex(idx int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.pipelineRetries[idx]++
	p.currentIndex = p.stableIndex
}

// SetToRetryIndex accepts a param and sets the CurrentIndex to this index value
func (p *PipelineSelector) setToCurrentIndex(index int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.currentIndex = index
}

// MaxRetriesUsed exported access to maxRetriesUsed
func (p *PipelineSelector) MaxRetriesUsed(idx int) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.pipelineRetries[idx] >= p.maxRetry
}

// SetNewStableIndex Update stableIndex to the passed stable index
func (p *PipelineSelector) setNewStableIndex(idx int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.pipelineRetries[idx] = 0
	p.stableIndex = idx
}

// IndexIsStable returns if index passed is the stable index
func (p *PipelineSelector) IndexIsStable(idx int) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.stableIndex == idx
}

func (p *PipelineSelector) StableIndex() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.stableIndex
}

func (p *PipelineSelector) CurrentIndex() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.currentIndex
}

func (p *PipelineSelector) IndexRetryCount(idx int) int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.pipelineRetries[idx]
}

// reportStable reports back to the failoverRouter that the current priority level that was called by Consume.SIGNAL was
// stable
func (p *PipelineSelector) ReportStable(idx int) {
	// is stableIndex is already the known stableIndex return
	if p.IndexIsStable(idx) {
		return
	}
	// if the stableIndex is a retried index, the update the stable index to the retried index
	// NOTE retry will not stop due to potential higher priority index still available
	p.setNewStableIndex(idx)
}

func NewPipelineSelector(lenPriority int, maxRetries int) *PipelineSelector {
	return &PipelineSelector{
		currentIndex:    0,
		stableIndex:     0,
		lock:            sync.RWMutex{},
		pipelineRetries: make([]int, lenPriority),
		maxRetry:        maxRetries,
	}
}
