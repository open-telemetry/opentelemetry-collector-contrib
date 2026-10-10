// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"sync"
	"time"
)

// staggeredDecisionTicker delays the first decision tick, then emits on a
// fixed period with no further phase adjustments. When firstDelay is 0 it
// matches time.NewTicker: the first tick arrives after one full period.
type staggeredDecisionTicker struct {
	C <-chan time.Time

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func newStaggeredDecisionTicker(firstDelay, period time.Duration) *staggeredDecisionTicker {
	out := make(chan time.Time)
	t := &staggeredDecisionTicker{
		C:      out,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go t.run(out, firstDelay, period)
	return t
}

func (t *staggeredDecisionTicker) run(out chan time.Time, firstDelay, period time.Duration) {
	defer close(t.doneCh)
	defer close(out)

	send := func(tm time.Time) bool {
		select {
		case out <- tm:
			return true
		case <-t.stopCh:
			return false
		}
	}

	if firstDelay > 0 {
		timer := time.NewTimer(firstDelay)
		select {
		case tm := <-timer.C:
			if !send(tm) {
				return
			}
		case <-t.stopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}

	if period <= 0 {
		return
	}

	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case tm := <-ticker.C:
			if !send(tm) {
				return
			}
		case <-t.stopCh:
			return
		}
	}
}

// Stop ends the ticker goroutine and waits for it to exit. It is safe to call
// more than once.
func (t *staggeredDecisionTicker) Stop() {
	t.stopOnce.Do(func() {
		close(t.stopCh)
		<-t.doneCh
	})
}

// decisionTickDelay returns the delay before shardIndex's first decision tick
// for numShards evenly spaced across period, offset by base.
func decisionTickDelay(shardIndex, numShards int, period, base time.Duration) time.Duration {
	if numShards <= 0 || period <= 0 {
		return 0
	}
	return base + time.Duration(shardIndex)*period/time.Duration(numShards)
}

// assignDecisionTickDelays sets each shard's firstTickDelay so ticks are evenly
// staggered across window, starting from base. Shards that already have a fixed
// delay (tests) are left unchanged.
func assignDecisionTickDelays(shards []*tailSamplingSpanProcessor, base, window time.Duration) {
	n := len(shards)
	if n == 0 || window <= 0 {
		return
	}
	for i, s := range shards {
		if s.fixedFirstTickDelay {
			continue
		}
		s.firstTickDelay = decisionTickDelay(i, n, window, base)
	}
}
