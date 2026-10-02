// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"sync"

	"golang.org/x/time/rate"
)

// Limiter is a token bucket that evaluators on different shards can share
// to enforce one limit together.
type Limiter struct {
	*rate.Limiter
	// Mutex serializes reading the available budget with spending it, so
	// concurrent budgetLimiters cannot both spend the same tokens.
	sync.Mutex
}

// NewLimiter returns a full Limiter refilling at perSecond. An unset burst
// capacity defaults to 2x perSecond.
func NewLimiter(perSecond, burstCapacity int64) *Limiter {
	if burstCapacity <= 0 {
		burstCapacity = 2 * perSecond
	}
	return &Limiter{Limiter: rate.NewLimiter(rate.Limit(perSecond), int(burstCapacity))}
}
