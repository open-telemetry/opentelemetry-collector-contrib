// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storm // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/storm"

import (
	"sync"
	"time"
)

// New returns a Limiter configured with the provided caps.
// Signature matches tests: New(transitionsPerMinute, eventsPerInterval, interval)
func New(maxTransitionsPerMinute, maxEventsPerInterval int, interval time.Duration) *Limiter {
	return &Limiter{
		MaxTransitionsPerMinute: maxTransitionsPerMinute,
		MaxEventsPerInterval:    maxEventsPerInterval,
		Interval:                interval,
	}
}

// Limiter applies coarse global throttles for alert storms.
// - MaxEventsPerInterval: maximum number of events allowed within each Interval window.
// - MaxTransitionsPerMinute: maximum number of "transition" events (isTransition=true) allowed per rolling minute.
type Limiter struct {
	MaxTransitionsPerMinute int
	MaxEventsPerInterval    int
	Interval                time.Duration

	mu sync.Mutex

	// per-Interval accounting
	intervalStart  time.Time
	eventsInWindow int

	// per-minute transition accounting
	minuteStart       time.Time
	transitionsInSlab int
}

// resetIntervalIfNeeded ensures we are counting within the current Interval.
func (l *Limiter) resetIntervalIfNeeded(now time.Time) {
	if l.Interval <= 0 {
		// Treat as "no per-interval limit" by keeping a single window with infinite duration.
		return
	}
	if l.intervalStart.IsZero() {
		l.intervalStart = now
		l.eventsInWindow = 0
		return
	}
	if now.Sub(l.intervalStart) >= l.Interval {
		l.intervalStart = now
		l.eventsInWindow = 0
	}
}

// resetMinuteIfNeeded ensures we are counting within the current minute slab for transitions.
func (l *Limiter) resetMinuteIfNeeded(now time.Time) {
	if l.MaxTransitionsPerMinute <= 0 {
		// No transition limiting requested.
		return
	}
	if l.minuteStart.IsZero() {
		l.minuteStart = now
		l.transitionsInSlab = 0
		return
	}
	if now.Sub(l.minuteStart) >= time.Minute {
		l.minuteStart = now
		l.transitionsInSlab = 0
	}
}

// Allow decides how many out of n events can proceed, and how many must be dropped.
// If isTransition is true, transition-per-minute limits also apply.
func (l *Limiter) Allow(n int, isTransition bool) (allow, dropped int) {
	if n <= 0 {
		return 0, 0
	}

	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Keep windows fresh
	l.resetIntervalIfNeeded(now)
	l.resetMinuteIfNeeded(now)

	allow = n

	// 1) Apply per-interval events limit
	if l.MaxEventsPerInterval > 0 && l.Interval > 0 {
		remaining := l.MaxEventsPerInterval - l.eventsInWindow
		if remaining < 0 {
			remaining = 0
		}
		if allow > remaining {
			allow = remaining
		}
	}

	// 2) Apply per-minute transitions limit (only for transition events)
	if isTransition && l.MaxTransitionsPerMinute > 0 {
		remaining := l.MaxTransitionsPerMinute - l.transitionsInSlab
		if remaining < 0 {
			remaining = 0
		}
		if allow > remaining {
			allow = remaining
		}
	}

	if allow < 0 {
		allow = 0
	}
	dropped = n - allow

	// Update counters with what we actually allow
	if allow > 0 {
		l.eventsInWindow += allow
		if isTransition && l.MaxTransitionsPerMinute > 0 {
			l.transitionsInSlab += allow
		}
	}

	return allow, dropped
}
