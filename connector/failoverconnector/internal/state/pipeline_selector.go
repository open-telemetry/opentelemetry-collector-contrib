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
	if idx != p.currentPipeline {
		return
	}
	p.NextStableLevel()
	p.TryEnableRetry()
}

// NextStableLevel increments the level to the next in the priority list
func (p *PipelineSelector) NextStableLevel() {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.currentPipeline++
}

// TryEnableRetry checks if a retry is already in effect and if not starts the retry goroutine
func (p *PipelineSelector) TryEnableRetry() {
	select {
	case <-p.retryEnabledToken:
		p.LaunchRetry()
	default:
	}
}

// LaunchRetry invokes the goroutine responsible for notifying the failover component to retry
func (p *PipelineSelector) LaunchRetry() {
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
