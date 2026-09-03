// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rollingspanlatencyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/rollingspanlatencyprocessor"

import (
	"math"
	"sync"
	"time"
)

// spanStats maintains an exponentially weighted mean and variance for a single
// span name. The decay factor alpha is derived from the configured half-life:
//
//	alpha = 1 - exp(-ln(2) * dt / halfLife)
//
// where dt is the elapsed time since the last observation. This makes the
// effective weight of any sample halve every halfLife regardless of observation
// frequency.
type spanStats struct {
	lastSeen time.Time
	mean     float64
	variance float64
	count    int64
	mu       sync.RWMutex
}

// update incorporates a new duration sample (nanoseconds) at the given wall
// time, returning the current mean and stddev after the update.
func (s *spanStats) update(durationNs float64, now time.Time, halfLife time.Duration) (mean, stddev float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alpha := s.decayAlpha(now, halfLife)
	s.lastSeen = now
	s.count++

	if s.count == 1 {
		s.mean = durationNs
		s.variance = 0
		return s.mean, 0
	}

	diff := durationNs - s.mean
	s.mean += alpha * diff
	// EWMA variance: V_t = (1-α)*(V_{t-1} + α*diff²)
	s.variance = (1 - alpha) * (s.variance + alpha*diff*diff)

	return s.mean, math.Sqrt(s.variance)
}

// snapshot returns the current mean and stddev without updating.
func (s *spanStats) snapshot() (mean, stddev float64, count int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mean, math.Sqrt(s.variance), s.count
}

// idleSince returns the time of the most recent observation, used by the
// eviction sweep to determine whether the entry has gone stale.
func (s *spanStats) idleSince() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSeen
}

// decayAlpha computes alpha based on elapsed time since lastSeen. For the very
// first call (lastSeen.IsZero) it returns 1.0 so the first sample seeds the
// mean directly.
func (s *spanStats) decayAlpha(now time.Time, halfLife time.Duration) float64 {
	if s.lastSeen.IsZero() {
		return 1.0
	}
	dt := now.Sub(s.lastSeen).Seconds()
	hl := halfLife.Seconds()
	return 1.0 - math.Exp(-math.Ln2*dt/hl)
}
