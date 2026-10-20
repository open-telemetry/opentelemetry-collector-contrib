// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"
)

// fleetTracker reports the live number of collector instances in this
// collector's fleet. The processor resolves the extension named by the
// fleet_tracker setting against this interface structurally via
// host.GetExtensions, so extension implementations do not need to import
// this package. The method signature below is the whole contract, and it is
// deliberately unexported: nothing needs to import it to satisfy it. If a
// compile-time assertion ever becomes worth offering, the interface can move
// to its own small module in the style of pkg/sampling without changing any
// implementation.
type fleetTracker interface {
	// SubscribeMemberCount registers callback to receive the fleet's live
	// member count. The callback is invoked once with the current count when
	// the subscription is established, and again on every change. Callbacks
	// must not block. The returned cancel func unregisters the callback and
	// must be safe to call once. Well-behaved implementations invoke
	// callbacks serially with respect to each other, but the processor also
	// tolerates concurrent delivery.
	SubscribeMemberCount(callback func(count int)) (cancel func(), err error)
}

// fleetState owns the processor's fleet_tracker runtime state and applies
// reported member counts to the adaptive_throughput rules' samplers. It lives
// on the processor as a single field so all fleet wiring stays in one place.
type fleetState struct {
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
	rules     []*rule

	// hasThroughputRules is true when at least one rule uses
	// adaptive_throughput, computed once in newProcessor so Start does not
	// need to rescan the rules to decide whether to subscribe.
	hasThroughputRules bool

	// cancel unsubscribes from the fleet_tracker extension. Set in Start,
	// invoked in Shutdown.
	cancel func()

	// mu guards size, bad, and clamped. Deliberately separate from the
	// processor's decision mutex: onMemberCount never touches the decision
	// path, so holding mu across a whole callback's work never contends with
	// decisions. It is held across the whole non-error path of onMemberCount
	// (the compare against size, the store, the setter calls, and the gauge
	// record) so two concurrent callbacks cannot store and apply their counts
	// in opposite orders, a divergence the setter dedup would otherwise make
	// permanent.
	mu sync.Mutex
	// size is the last good member count reported by fleet_tracker,
	// initialized to 1 in newProcessor.
	size int
	// bad tracks whether the most recent callback reported a non-positive
	// count, so the warning in onMemberCount logs only on the transition into
	// that state.
	bad bool
	// clamped tracks, per rule name, whether that rule's fleet-divided goal
	// is currently clamped to the 1/s floor, so the over-delivery warning
	// logs only on the transition into that state.
	clamped map[string]bool
}

// onMemberCount applies a newly reported fleet member count to every
// adaptive_throughput rule's sampler, dividing each rule's configured
// goal_throughput by the count. Registered as the fleet_tracker subscription
// callback; must not block.
func (f *fleetState) onMemberCount(count int) {
	ctx := context.Background()
	if count <= 0 {
		f.telemetry.ProcessorAdaptiveTailSamplingFleetTrackerErrors.Add(ctx, 1)
		f.mu.Lock()
		firstBad := !f.bad
		f.bad = true
		f.mu.Unlock()
		if firstBad {
			f.logger.Warn("fleet tracker reported a non-positive member count; keeping the last good count", zap.Int("member_count", count))
		}
		return // last good N kept implicitly: samplers retain their current goal
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.bad {
		f.bad = false
		f.logger.Debug("fleet tracker recovered with a positive member count", zap.Int("member_count", count))
	}

	// The setter dedup below only guards SetGoalThroughputPerSec; the gauge is
	// recorded on every callback regardless, so a fleet that reports the same
	// N repeatedly (or only ever delivers its initial N) still emits
	// fleet_member_count.
	applyGoals := count != f.size
	f.size = count

	for _, r := range f.rules {
		if r.goalThroughput <= 0 {
			continue
		}
		goal := max(r.goalThroughput/count, 1)
		clamped := r.goalThroughput/count < 1
		if clamped && !f.clamped[r.name] {
			f.logger.Warn(
				"adaptive_throughput rule's fleet-divided goal is clamped to 1/s; the fleet now emits more than the configured budget",
				zap.String("rule", r.name), zap.Int("goal_throughput", r.goalThroughput), zap.Int("member_count", count),
			)
		}
		if clamped {
			if f.clamped == nil {
				f.clamped = make(map[string]bool)
			}
			f.clamped[r.name] = true
		} else {
			delete(f.clamped, r.name)
		}

		if applyGoals {
			// The !ok case (sampler does not implement ThroughputGoalSetter)
			// is already warned once at Start; staying silent here avoids
			// spamming that warning on every callback.
			if setter, ok := r.sampler.(sampler.ThroughputGoalSetter); ok {
				setter.SetGoalThroughputPerSec(goal)
				f.logger.Debug("applied fleet-divided goal",
					zap.String("rule", r.name), zap.Int("goal_per_sec", goal), zap.Int("member_count", count))
			}
		}
	}
	f.telemetry.ProcessorAdaptiveTailSamplingFleetMemberCount.Record(ctx, int64(count))
}

// resolveFleetTracker resolves the extension named by id against host, and
// asserts it satisfies the fleet_tracker contract.
func resolveFleetTracker(host component.Host, id component.ID) (fleetTracker, error) {
	if host == nil {
		return nil, errors.New("fleet_tracker configured but host is nil")
	}
	extension, ok := host.GetExtensions()[id]
	if !ok {
		return nil, fmt.Errorf("fleet_tracker extension %q not found", id)
	}
	tracker, ok := extension.(fleetTracker)
	if !ok {
		return nil, fmt.Errorf("extension %q does not implement SubscribeMemberCount, so it cannot be used as a fleet_tracker", id)
	}
	return tracker, nil
}
