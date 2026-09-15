// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/samplingstate"
)

// runCounterSync drives one throughput sampler's interval loop: every
// adjustment interval it publishes this instance's counts to the counter
// store, reads back the merged totals, and folds them into the rate engine.
// Ticks are aligned to wall-clock interval boundaries so instances sharing a
// store address the same bucket for the same time period and publish at
// roughly the same moment.
func (p *adaptiveTailSamplingProcessor) runCounterSync(ctx context.Context, name string, st *sampler.SharedThroughput, store samplingstate.CounterStore, timeout time.Duration) {
	defer p.syncWG.Done()
	interval := st.Interval()
	if timeout <= 0 {
		// Default to half the interval: short enough that a stuck store cannot
		// eat a whole tick, long enough to tolerate normal store latency.
		timeout = interval / 2
	}
	timer := time.NewTimer(time.Until(nextBoundary(time.Now(), interval)))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-timer.C:
			p.syncCounters(ctx, now, name, st, store, timeout)
			timer.Reset(time.Until(nextBoundary(time.Now(), interval)))
		}
	}
}

// nextBoundary returns the first wall-clock multiple of interval after now.
func nextBoundary(now time.Time, interval time.Duration) time.Time {
	return now.Truncate(interval).Add(interval)
}

// syncCounters runs one tick: publish the counts accumulated over the
// just-completed bucket, read the merged totals for that bucket, and apply
// them. A peer publishing slightly later than this instance reads is missed
// for that bucket; the engines smooth over that skew and it corrects on the
// next tick.
//
// Store failures fail open: the local counts are applied instead, degrading
// to per-instance rates against the full goal (over-sampling across the
// fleet) rather than stalling the sampler. Each store call is bounded by
// timeout, so a store slower than the interval routes into the same fail-open
// path rather than blocking the loop. The decide path never touches the
// store, so it is unaffected either way.
func (p *adaptiveTailSamplingProcessor) syncCounters(ctx context.Context, now time.Time, name string, st *sampler.SharedThroughput, store samplingstate.CounterStore, timeout time.Duration) {
	interval := st.Interval()
	// The tick fires at (or just after) a bucket boundary; stepping back half
	// an interval indexes the bucket that just completed.
	bucket := now.Add(-interval/2).UnixNano() / interval.Nanoseconds()
	counts := st.SnapshotCounts()
	if len(counts) > 0 {
		if err := p.withTimeout(ctx, timeout, func(cctx context.Context) error {
			return store.AddCounts(cctx, name, bucket, counts)
		}); err != nil {
			p.recordCounterSyncError(ctx, name, "add", err)
			st.ApplyMergedCounts(counts)
			return
		}
	}
	var merged map[string]float64
	if err := p.withTimeout(ctx, timeout, func(cctx context.Context) error {
		var err error
		merged, err = store.ReadCounts(cctx, name, bucket)
		return err
	}); err != nil {
		p.recordCounterSyncError(ctx, name, "read", err)
		st.ApplyMergedCounts(counts)
		return
	}
	st.ApplyMergedCounts(merged)
}

// withTimeout runs fn under a child context bounded by timeout so a slow store
// call is abandoned rather than stalling the sync loop.
func (*adaptiveTailSamplingProcessor) withTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(cctx)
}

func (p *adaptiveTailSamplingProcessor) recordCounterSyncError(ctx context.Context, name, op string, err error) {
	p.telemetry.ProcessorAdaptiveTailSamplingCounterSyncErrors.Add(ctx, 1,
		metric.WithAttributes(attribute.String("rule", name), attribute.String("op", op)))
	p.logger.Warn("counter store sync failed; applying this instance's own counts",
		zap.String("rule", name),
		zap.String("op", op),
		zap.Error(err))
}
