// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// GetStochasticAdjustedCount returns the stochastic-rounded adjusted count for the span.
// The second return value indicates whether the count is adjusted (i.e., the span has
// a valid tracestate with a sampling threshold). When false, the count will be 1, meaning it only represents the span itself.
func GetStochasticAdjustedCount(span *ptrace.Span) (uint64, bool) {
	tracestate := span.TraceState().AsRaw()
	w3cTraceState, err := sampling.NewW3CTraceState(tracestate)
	if err != nil {
		return 1, false
	}
	adjustedCount := w3cTraceState.OTelValue().AdjustedCount()
	if adjustedCount == 0 {
		return 1, false
	}
	return stochasticIncrement(adjustedCount), true
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

func stochasticIncrement(weight float64) uint64 {
	floor, frac := math.Modf(weight)
	count := uint64(floor)
	if frac <= 0 {
		return count
	}
	rng := prngPool.Get().(*xorshift64star)
	defer prngPool.Put(rng)

	threshold := uint64(frac * float64(math.MaxUint64))
	if rng.next() < threshold {
		count++
	}
	return count
}
