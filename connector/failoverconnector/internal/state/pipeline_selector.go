// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"sync"
)

// PipelineSelector is meant to serve as the source of truth for the target priority level
type PipelineSelector struct {
	CurrentIndex    int
	StableIndex     int
	lock            sync.RWMutex
	PipelineRetries []int
	maxRetry        int
}

// UpdatePipelineIndex is the main function that updates the pipeline indexes due to an error
// if the currentIndex is not the stableIndex, that means the currentIndex is a higher
// priority index that was set during a retry, in which case we return to the stable index
func (p *PipelineSelector) UpdatePipelineIndex(idx int) {
	if p.IndexIsStable(idx) {
		p.SetToNextPriorityPipeline()
	}
	p.SetToStableIndex()
}

// NextPipeline skips through any lower priority pipelines that have exceeded their maxRetries
// and sets the first that has not as the new stable
func (p *PipelineSelector) SetToNextPriorityPipeline() {
	p.lock.Lock()
	defer p.lock.Unlock()
	for ok := true; ok; ok = p.exceededMaxRetries() {
		p.CurrentIndex++
	}
	p.StableIndex = p.CurrentIndex
}

func (p *PipelineSelector) exceededMaxRetries() bool {
	return p.CurrentIndex < len(p.PipelineRetries) && (p.PipelineRetries[p.CurrentIndex] >= p.maxRetry)
}

// SetToStableIndex returns the CurrentIndex to the known Stable Index
func (p *PipelineSelector) SetToStableIndex() {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.PipelineRetries[p.CurrentIndex]++
	p.CurrentIndex = p.StableIndex
}

// SetToRetryIndex accepts a param and sets the CurrentIndex to this index value
func (p *PipelineSelector) SetToRetryIndex(index int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.CurrentIndex = index
}

// MaxRetriesUsed exported access to maxRetriesUsed
func (p *PipelineSelector) MaxRetriesUsed(index int) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.PipelineRetries[index] >= p.maxRetry
}

// SetNewStableIndex Update stableIndex to the passed stable index
func (p *PipelineSelector) SetNewStableIndex(idx int) {
	p.lock.Lock()
	defer p.lock.RUnlock()
	p.PipelineRetries[p.CurrentIndex] = 0
	p.StableIndex = idx
}

// IndexIsStable returns if index passed is the stable index
func (p *PipelineSelector) IndexIsStable(idx int) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return idx == p.StableIndex
}

func (p *PipelineSelector) GetStableIndex() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.StableIndex
}

func (p *PipelineSelector) GetCurrentIndex() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.CurrentIndex
}

func NewPipelineSelector(lenPriority int, maxRetries int) *PipelineSelector {
	return &PipelineSelector{
		CurrentIndex:    0,
		StableIndex:     0,
		lock:            sync.RWMutex{},
		PipelineRetries: make([]int, lenPriority),
		maxRetry:        maxRetries,
	}
}
