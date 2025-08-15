// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracelimiter

import (
	"testing"
	"time"
)

func TestBlockingTracesLimiter(t *testing.T) {
	limiter := NewBlockingTracesLimiter(2)

	done := make(chan struct{})
	blocked := make(chan struct{})

	// Accept first trace, should not block
	go func() {
		limiter.AcceptTrace(t.Context(), [16]byte{}, time.Now())
		done <- struct{}{}
	}()
	<-done

	// Accept second trace, should not block
	go func() {
		limiter.AcceptTrace(t.Context(), [16]byte{}, time.Now())
		done <- struct{}{}
	}()
	<-done

	// Accept third trace, should block until OnDeleteTrace is called
	go func() {
		limiter.AcceptTrace(t.Context(), [16]byte{}, time.Now())
		blocked <- struct{}{}
	}()

	// Give goroutine time to potentially block
	select {
	case <-blocked:
		t.Fatal("AcceptTrace should have blocked, but it did not")
	case <-time.After(100 * time.Millisecond):
		// Expected: still blocked after timeout
	}

	// Now free up a slot
	limiter.OnDeleteTrace()

	// Now the blocked AcceptTrace should proceed
	select {
	case <-blocked:
		// Success: unblocked as expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("AcceptTrace should have unblocked after OnDeleteTrace")
	}
}
