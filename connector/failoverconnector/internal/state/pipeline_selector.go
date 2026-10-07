// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"sync"
	"time"
)

type PipelineSelector struct {
	currentPipeline   int
	constants         PSConstants
	lock              sync.RWMutex
	retryEnabledToken chan struct{}
	retryChan         chan<- struct{}

	retryCancel CancelManager
	done        chan struct{}
}

// HandleError is called when an error is returned on a healthy pipeline
func (p *PipelineSelector) HandleError(idx int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if idx != p.currentPipeline {
		return
	}
	p.currentPipeline++
	p.tryEnableRetry()
}

func (p *PipelineSelector) tryEnableRetry() {
	select {
	case <-p.done:
		return
	default:
	}
	select {
	case <-p.retryEnabledToken:
	default:
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.retryCancel.UpdateFn(cancel)

	go func() {
		ticker := time.NewTicker(p.constants.RetryInterval)
		defer func() {
			ticker.Stop()
			p.returnRetryToken()
		}()
		for {
			select {
			case <-ticker.C:
				select {
				case p.retryChan <- struct{}{}:
				default:
				}
			case <-ctx.Done():
				return
			case <-p.done:
				return
			}
		}
	}()
}

// returnRetryToken returns the token back to the buffered channel allowing the next retry function to consume the token
func (p *PipelineSelector) returnRetryToken() {
	p.retryEnabledToken <- struct{}{}
}

// CurrentLevel returns the current healthy pipeline level
func (p *PipelineSelector) CurrentPipeline() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.currentPipeline
}

// ResetHealthyPipeline resets a pipeline level that was successfully retries back to healthy/active
func (p *PipelineSelector) ResetHealthyPipeline(pipelineIndex int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if pipelineIndex == 0 {
		p.retryCancel.Cancel()
		// Without this wait, a new failure can miss the token and leave retries stopped.
		<-p.retryEnabledToken
		p.returnRetryToken()
	}
	p.currentPipeline = pipelineIndex
}

func NewPipelineSelector(retryChan chan<- struct{}, done chan struct{}, consts PSConstants) *PipelineSelector {
	retryEnabledToken := make(chan struct{}, 1)
	retryEnabledToken <- struct{}{}

	ps := &PipelineSelector{
		currentPipeline:   0,
		constants:         consts,
		retryEnabledToken: retryEnabledToken,
		retryChan:         retryChan,
		done:              done,
	}
	return ps
}

// For Testing
func (p *PipelineSelector) TestSetCurrentPipeline(idx int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.currentPipeline = idx
}
