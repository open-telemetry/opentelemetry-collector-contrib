// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logratelimitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logratelimitprocessor"

import (
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiter is a fixedWindow algorithm based rate-limiter
type RateLimiter struct {
	threshold          uint64
	windowStartTimeMap sync.Map
	windowSize         time.Duration
	counterMap         sync.Map
	logger             *zap.Logger
}

// NewRateLimiter create a new rate-limiter initialization
func NewRateLimiter(threshold uint64, windowSize time.Duration, lggr *zap.Logger) *RateLimiter {
	return &RateLimiter{
		threshold:  threshold,
		windowSize: windowSize,
		logger:     lggr,
	}
}

// IsRequestAllowed checks if a log record should be allowed or drop instead wrt configured AllowedRate in configured interval
// the algorithm works on best-effort basis and do not involve any mutex locking, instead it allows race-condition to happen
// and try to drop logs in the best effort way, i.e. it is possible that algorithm might allow slightly more or less logs than what
// is configured in AllowedRate
func (fw *RateLimiter) IsRequestAllowed(key uint64) bool {
	now := time.Now()
	cntrI, _ := fw.counterMap.LoadOrStore(key, new(atomic.Uint64))
	cntr := cntrI.(*atomic.Uint64)
	currWinI, _ := fw.windowStartTimeMap.LoadOrStore(key, now)
	currWin := currWinI.(time.Time)

	// cases where now.Sub(fw.windowStartTime) is negative or positive should be handled
	if now.Sub(currWin) > fw.windowSize {
		cntr.Store(0)
		// updating this without lock considering an assumption that all goroutines which will try to update this
		// fw.windowStartTime in race-condition(within this if block) will have a very small time-difference (in nanoseconds),
		// we can tolerate that time difference and accept any of the update from concurrent requests.
		fw.windowStartTimeMap.Store(key, now)
	}
	return cntr.Add(1) <= fw.threshold
}
