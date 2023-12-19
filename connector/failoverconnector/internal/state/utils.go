// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"sync"
)

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
