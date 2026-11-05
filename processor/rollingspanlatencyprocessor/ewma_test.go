// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rollingspanlatencyprocessor

import (
	"math"
	"testing"
	"time"
)

func TestSpanStats_FirstSampleSeedsMean(t *testing.T) {
	s := &spanStats{}
	now := time.Now()
	mean, stddev := s.update(100e6, now, 2*time.Hour)

	if mean != 100e6 {
		t.Errorf("want mean=100e6, got %v", mean)
	}
	if stddev != 0 {
		t.Errorf("want stddev=0 on first sample, got %v", stddev)
	}
}

func TestSpanStats_MeanConvergesToSteadyState(t *testing.T) {
	s := &spanStats{}
	now := time.Now()
	// Feed 200 samples of 100ms, spaced 1s apart.
	// Mean should converge near 100ms.
	for range 200 {
		now = now.Add(time.Second)
		s.update(100e6, now, 2*time.Hour)
	}
	mean, _, _ := s.snapshot()
	if math.Abs(mean-100e6)/100e6 > 0.01 {
		t.Errorf("mean did not converge to 100ms: got %.2fms", mean/1e6)
	}
}

func TestSpanStats_HalfLifeDecay(t *testing.T) {
	// After feeding samples at value V for a long time then switching to 2V,
	// the mean should approach 2V after another few half-lives worth of samples.
	s := &spanStats{}
	now := time.Now()
	const v = 100e6
	for range 500 {
		now = now.Add(time.Second)
		s.update(v, now, 2*time.Hour)
	}
	// Switch to 2*v; feed for 10 half-lives (20 hours at 1s intervals = 72000 steps).
	// After N half-lives the old mean's contribution is 2^-N of the total, so
	// at 10 half-lives the residual error is < 0.1%.
	for range 72000 {
		now = now.Add(time.Second)
		s.update(2*v, now, 2*time.Hour)
	}
	mean, _, _ := s.snapshot()
	// Should be within 1% of 200ms
	if math.Abs(mean-2*v)/(2*v) > 0.01 {
		t.Errorf("mean did not decay to new steady state: got %.2fms", mean/1e6)
	}
}

func TestSpanStats_VarianceGrowsWithSpread(t *testing.T) {
	s := &spanStats{}
	now := time.Now()
	// Alternating 50ms and 150ms — variance must be non-zero.
	for i := range 100 {
		now = now.Add(time.Second)
		val := 50e6
		if i%2 == 0 {
			val = 150e6
		}
		s.update(val, now, 2*time.Hour)
	}
	_, stddev, _ := s.snapshot()
	if stddev <= 0 {
		t.Error("stddev should be positive with spread data")
	}
}

// TestSpanStats_SameInstantBurstBuildsRealVariance simulates a batch of spans
// processed at the same wall-clock instant (dt=0 between updates, as happens
// when a single ConsumeTraces call advances the processing clock far less
// than one nanosecond between spans). With a pure time-decay alpha this
// degenerates to alpha≈0 for every sample after the first, freezing the
// baseline at the first sample's value regardless of how different the rest
// of the burst is. The count-floor (alpha = max(timeDecayAlpha, 1/count))
// must let a same-instant burst build a real running mean/variance instead.
func TestSpanStats_SameInstantBurstBuildsRealVariance(t *testing.T) {
	s := &spanStats{}
	now := time.Now()
	durations := []float64{100e6, 100e6, 100e6, 500e6, 100e6, 100e6, 500e6, 100e6, 100e6, 500e6}
	for _, d := range durations {
		s.update(d, now, 2*time.Hour) // now never advances: dt=0 for every update after the first
	}
	mean, stddev, _ := s.snapshot()

	// A plain running (unweighted) average/variance over the same samples is
	// the target: the burst should be scored like an ordinary sample set, not
	// pinned to the first observation.
	var wantMean float64
	for _, d := range durations {
		wantMean += d
	}
	wantMean /= float64(len(durations))

	if math.Abs(mean-wantMean)/wantMean > 0.01 {
		t.Errorf("same-instant burst mean should track the running average: got %.2fms, want ~%.2fms", mean/1e6, wantMean/1e6)
	}
	if stddev <= 0 {
		t.Errorf("same-instant burst with mixed durations must produce non-zero stddev, got %v", stddev)
	}
}

func TestDecayAlpha_ZeroElapsed(t *testing.T) {
	s := &spanStats{}
	now := time.Now()
	s.lastSeen = now
	alpha := s.decayAlpha(now, 2*time.Hour)
	// zero elapsed → alpha ≈ 0 (no update weight)
	if alpha > 1e-10 {
		t.Errorf("expected alpha≈0 for zero elapsed, got %v", alpha)
	}
}

func TestDecayAlpha_OneHalfLife(t *testing.T) {
	s := &spanStats{}
	now := time.Now()
	s.lastSeen = now
	alpha := s.decayAlpha(now.Add(2*time.Hour), 2*time.Hour)
	// After one half-life alpha = 1 - exp(-ln2) = 1 - 0.5 = 0.5
	if math.Abs(alpha-0.5) > 1e-9 {
		t.Errorf("expected alpha=0.5 at one half-life, got %v", alpha)
	}
}
