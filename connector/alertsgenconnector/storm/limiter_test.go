// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storm

import (
	"sync"
	"testing"
	"time"
)

// Helper to quickly create a limiter.
func newLimiter(t *testing.T, maxTransPerMin, maxEventsPerInterval int, interval time.Duration) *Limiter {
	t.Helper()
	return New(maxTransPerMin, maxEventsPerInterval, interval)
}

func TestAllow_NoLimits_AllowsEverything(t *testing.T) {
	l := newLimiter(t, 0, 0, 0)

	allow, drop := l.Allow(10, false)
	if allow != 10 || drop != 0 {
		t.Fatalf("expected allow=10, drop=0; got allow=%d, drop=%d", allow, drop)
	}

	// transitions ignored because transitions cap is 0 (disabled)
	allow, drop = l.Allow(7, true)
	if allow != 7 || drop != 0 {
		t.Fatalf("expected allow=7, drop=0; got allow=%d, drop=%d", allow, drop)
	}
}

func TestAllow_PerIntervalLimit_ResetsAfterInterval(t *testing.T) {
	// Window size tiny to keep test fast.
	l := newLimiter(t, 0, 5, 40*time.Millisecond)

	// First call consumes 3/5
	allow, drop := l.Allow(3, false)
	if allow != 3 || drop != 0 {
		t.Fatalf("first: expected allow=3, drop=0; got allow=%d, drop=%d", allow, drop)
	}

	// Second call asks for 3, but only 2 remain in current window.
	allow, drop = l.Allow(3, false)
	if allow != 2 || drop != 1 {
		t.Fatalf("second: expected allow=2, drop=1; got allow=%d, drop=%d", allow, drop)
	}

	// Sleep past interval to cause window reset.
	time.Sleep(60 * time.Millisecond)

	// After reset, full 5 should be available again.
	allow, drop = l.Allow(4, false)
	if allow != 4 || drop != 0 {
		t.Fatalf("after reset: expected allow=4, drop=0; got allow=%d, drop=%d", allow, drop)
	}
}

func TestAllow_TransitionsPerMinuteLimit(t *testing.T) {
	// Transition cap = 3; interval cap very high so it doesn't interfere.
	l := newLimiter(t, 3, 1000, 100*time.Millisecond)

	// Ask for 5 transition events; only 3 allowed in current minute slab.
	allow, drop := l.Allow(5, true)
	if allow != 3 || drop != 2 {
		t.Fatalf("transitions: expected allow=3, drop=2; got allow=%d, drop=%d", allow, drop)
	}

	// Non-transition events are unaffected by the transitions cap.
	allow, drop = l.Allow(5, false)
	if allow != 5 || drop != 0 {
		t.Fatalf("non-transition should not be limited by transitions cap; got allow=%d, drop=%d", allow, drop)
	}
}

func TestAllow_BothLimits_MinimumWins(t *testing.T) {
	// Per-interval cap 5; transitions cap 10 (higher)
	// Expect per-interval to be the binding constraint for transition events here.
	l := newLimiter(t, 10, 5, 100*time.Millisecond)

	allow, drop := l.Allow(8, true)
	if allow != 5 || drop != 3 {
		t.Fatalf("expected per-interval to bind: allow=5, drop=3; got allow=%d, drop=%d", allow, drop)
	}

	// Consume remaining space in window: none should remain now.
	allow, drop = l.Allow(1, false)
	if allow != 0 || drop != 1 {
		t.Fatalf("window full: expected allow=0, drop=1; got allow=%d, drop=%d", allow, drop)
	}
}

func TestAllow_DisablePerIntervalWithZeroInterval(t *testing.T) {
	// MaxEventsPerInterval is set, but Interval=0 disables per-interval limiting.
	l := newLimiter(t, 0, 5, 0)

	allow, drop := l.Allow(20, false)
	if allow != 20 || drop != 0 {
		t.Fatalf("per-interval disabled via Interval=0; expected allow=20, drop=0; got allow=%d, drop=%d", allow, drop)
	}
}

func TestAllow_NonPositiveN_NoOp(t *testing.T) {
	l := newLimiter(t, 1, 1, 10*time.Millisecond)

	allow, drop := l.Allow(0, false)
	if allow != 0 || drop != 0 {
		t.Fatalf("n<=0 should no-op; got allow=%d, drop=%d", allow, drop)
	}

	allow, drop = l.Allow(-5, true)
	if allow != 0 || drop != 0 {
		t.Fatalf("n<=0 should no-op; got allow=%d, drop=%d", allow, drop)
	}
}

func TestAllow_ConcurrencySafe_TotalDoesNotExceedPerIntervalCap(t *testing.T) {
	// Cap = 100 per interval; run many concurrent calls quickly within a single window.
	capPerWindow := 100
	l := newLimiter(t, 0, capPerWindow, 150*time.Millisecond)

	var wg sync.WaitGroup
	var mu sync.Mutex
	totalAllowed := 0
	totalDropped := 0

	// 10 goroutines, each trying to send 15 events in the same window (total requested 150).
	const goroutines = 10
	const perG = 15

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			a, d := l.Allow(perG, false)
			mu.Lock()
			totalAllowed += a
			totalDropped += d
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Sum of allowed should never exceed the per-interval cap.
	if totalAllowed > capPerWindow {
		t.Fatalf("allowed exceeded cap: allowed=%d cap=%d", totalAllowed, capPerWindow)
	}

	// Sanity: allowed + dropped should equal total requested.
	totalRequested := goroutines * perG
	if totalAllowed+totalDropped != totalRequested {
		t.Fatalf("accounting mismatch: allowed+dropped=%d, requested=%d", totalAllowed+totalDropped, totalRequested)
	}
}

func TestAllow_TransitionsCap_BindsIndependentlyOfPerInterval(t *testing.T) {
	// transitions cap = 4; per-interval cap = very large so it won't bind
	l := newLimiter(t, 4, 1000, 200*time.Millisecond)

	// First burst consumes 3 transitions
	allow, drop := l.Allow(3, true)
	if allow != 3 || drop != 0 {
		t.Fatalf("first burst: expected allow=3, drop=0; got allow=%d, drop=%d", allow, drop)
	}

	// Next asks for 3 transitions, but only 1 remains in minute slab.
	allow, drop = l.Allow(3, true)
	if allow != 1 || drop != 2 {
		t.Fatalf("second burst: expected allow=1, drop=2; got allow=%d, drop=%d", allow, drop)
	}

	// Non-transition still free to pass (interval not binding)
	allow, drop = l.Allow(50, false)
	if allow != 50 || drop != 0 {
		t.Fatalf("non-transition unaffected: expected allow=50, drop=0; got allow=%d, drop=%d", allow, drop)
	}
}
