// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"sync"
	"time"
)

type PSConstants struct {
	RetryInterval time.Duration
	RetryGap      time.Duration
	MaxRetries    int
	RetryBackoff  time.Duration
}

type TryLock struct {
	lock sync.Mutex
}

func (t *TryLock) TryExecute(fn func(int), arg int) {
	if t.lock.TryLock() {
		defer t.lock.Unlock()
		fn(arg)
	}
}

func NewTryLock() *TryLock {
	return &TryLock{}
}

type cancelManager struct {
	cancelFunc context.CancelFunc
}

func (c *cancelManager) CancelAndSet(cancelFunc context.CancelFunc) {
	c.Cancel()
	c.cancelFunc = cancelFunc
}

func (c *cancelManager) Cancel() {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
}

// Manages cancel function for retry goroutine, ends up cleaner than using channels
type RetryState struct {
	lock sync.Mutex
	cancelManager
}
