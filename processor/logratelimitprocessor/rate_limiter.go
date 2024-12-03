// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logratelimitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logratelimitprocessor"

import (
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

// RateLimiter is a fixedWindow algorithm based rate-limiter
type RateLimiter struct {
	threshold          uint64
	windowStartTimeMap map[uint64]time.Time
	windowSize         time.Duration
	counterMap         map[uint64]*atomic.Uint64
	logger             *zap.Logger
}

// NewRateLimiter create a new rate-limiter initialization
func NewRateLimiter(threshold uint64, windowSize time.Duration, lggr *zap.Logger) *RateLimiter {
	return &RateLimiter{
		threshold:          threshold,
		windowStartTimeMap: make(map[uint64]time.Time),
		windowSize:         windowSize,
		counterMap:         make(map[uint64]*atomic.Uint64),
		logger:             lggr,
	}
}

// IsRequestAllowed checks if a log record should be allowed or drop instead wrt configured AllowedRate in configured interval
// the algorithm works on best-effort basis and do not involve any mutex locking, instead it allows race-condition to happen
// and try to drop logs in the best effort way, i.e. it is possible that algorithm might allow slightly more or less logs than what
// is configured in AllowedRate
func (fw *RateLimiter) IsRequestAllowed(key uint64) bool {
	now := time.Now()
	cntr, ok := fw.counterMap[key]
	if !ok {
		fw.counterMap[key] = new(atomic.Uint64)
		cntr = fw.counterMap[key]
		fw.windowStartTimeMap[key] = now
	}
	// cases where now.Sub(fw.windowStartTime) is negative or positive should be handled
	if now.Sub(fw.windowStartTimeMap[key]) > fw.windowSize {
		cntr.Store(0)
		// updating this without lock considering an assumption that all goroutines which will try to update this
		// fw.windowStartTime in race-condition(within this if block) will have a very small time-difference (in nanoseconds),
		// we can tolerate that time difference and accept any of the update from concurrent requests.
		fw.windowStartTimeMap[key] = now
	}
	return cntr.Add(1) <= fw.threshold
}
