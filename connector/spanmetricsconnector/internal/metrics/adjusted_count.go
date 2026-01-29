// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"

import (
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// AdjustedCountCache is a simple single-entry cache for adjusted count results.
// Use as a function-local variable to cache consecutive identical tracestates.
// No synchronization is needed when used within a single goroutine.
type AdjustedCountCache struct {
	tracestate string
	count      uint64
	isAdjusted bool
}

// NewAdjustedCountCache creates a cache initialized with correct defaults.
// The zero-value tracestate ("") maps to count=1, isAdjusted=false.
func NewAdjustedCountCache() AdjustedCountCache {
	return AdjustedCountCache{
		tracestate: "",
		count:      1,
		isAdjusted: false,
	}
}

// GetStochasticAdjustedCount returns the stochastic-rounded adjusted count for the span.
// The second return value indicates whether the count is adjusted (i.e., the span has
// a valid tracestate with a sampling threshold). When false, the count will be 1, meaning it only represents the span itself.
func GetStochasticAdjustedCount(span *ptrace.Span) (uint64, bool) {
	return GetStochasticAdjustedCountWithCache(span, nil)
}

// GetStochasticAdjustedCountWithCache is like GetStochasticAdjustedCount but accepts
// an optional cache. When processing spans in a batch, consecutive spans from the same
// trace will have identical tracestates, so caching avoids redundant parsing.
// Pass nil to disable caching.
// This method is thread-compatible.
func GetStochasticAdjustedCountWithCache(span *ptrace.Span, cache *AdjustedCountCache) (uint64, bool) {
	tracestate := span.TraceState().AsRaw()

	// Check cache first
	if cache != nil && tracestate == cache.tracestate {
		return cache.count, cache.isAdjusted
	}

	// Compute the adjusted count
	count, isAdjusted := computeAdjustedCount(tracestate)

	// Update cache
	if cache != nil {
		cache.tracestate = tracestate
		cache.count = count
		cache.isAdjusted = isAdjusted
	}

	return count, isAdjusted
}

// computeAdjustedCount does the actual computation without caching.
func computeAdjustedCount(tracestate string) (uint64, bool) {
	w3cTraceState, err := sampling.NewW3CTraceState(tracestate)
	if err != nil {
		return 1, false
	}
	threshold, exists := w3cTraceState.OTelValue().TValueThreshold()
	if !exists {
		return 1, false
	}
	denominator := sampling.MaxAdjustedCount - threshold.Unsigned()
	if denominator == 0 {
		return 1, false
	}
	return stochasticDiv(sampling.MaxAdjustedCount, denominator), true
}

// xorshift64star is a very fast PRNG using xor and shift.
type xorshift64star uint64

func (r *xorshift64star) next() uint64 {
	x := *r
	x ^= x << 12
	x ^= x >> 25
	x ^= x << 27
	*r = x
	return uint64(x) * 0x2545F4914F6CDD1D // The "Scrambler"
}

var seedCounter uint64

// prngPool is used to give each Goroutine its own PRNG state.
// This avoids global locks and cache-line bouncing.
var prngPool = sync.Pool{
	New: func() any {
		// Use a non-zero seed (crucial for xorshift) that is unique for each Goroutine.
		s := uint64(time.Now().UnixNano()) ^ atomic.AddUint64(&seedCounter, 1)
		if s == 0 {
			s = 1
		}
		seed := xorshift64star(s)
		return &seed
	},
}

// stochasticDiv computes numerator/denominator with stochastic rounding.
// It returns floor(numerator/denominator) and probabilistically adds 1 based on
// the remainder, ensuring an unbiased estimate over many calls.
// When the denominator is 0, the behavior is undefined.
func stochasticDiv(numerator, denominator uint64) uint64 {
	if denominator == 0 { // although the behavior is undefined, we return 0 to avoid panics.
		return 0
	}
	quotient := numerator / denominator
	remainder := numerator % denominator
	if remainder == 0 {
		return quotient
	}
	rng := prngPool.Get().(*xorshift64star)
	defer prngPool.Put(rng)

	// Round up with probability remainder/denominator
	if rng.next()%denominator < remainder {
		quotient++
	}
	return quotient
}
