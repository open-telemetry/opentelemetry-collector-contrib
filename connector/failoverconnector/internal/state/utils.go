// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"sync"
)

type TryLock struct {
	lock chan struct{}
}

func NewTryLock() *TryLock {
	return &TryLock{
		lock: make(chan struct{}, 1),
	}
}

// Lock tries to write to a channel of size 1 to maintain a single access point to a resource
// if not default case will return
// NOTE: may need to update logic in future so that concurrent calls to consume<SIGNAL> block while the lock is acquired
// and then return automatically once lock is released to avoid repeated calls to consume<SIGNAL> before indexes are updated
func (tl *TryLock) Lock(fn func(int), arg int) {
	select {
	case tl.lock <- struct{}{}:
		defer tl.Unlock()
		fn(arg)
	default:
	}
}

func (tl *TryLock) Unlock() {
	<-tl.lock
}

// Manages cancel function for retry goroutine, ends up cleaner than using channels
type RetryState struct {
	lock        sync.Mutex
	cancelRetry context.CancelFunc
}

func (m *RetryState) UpdateCancelFunc(newCancelFunc context.CancelFunc) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.cancelRetry = newCancelFunc
}

func (m *RetryState) InvokeCancel() {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.cancelRetry != nil {
		m.cancelRetry()
	}
}
