// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"sync/atomic"
	"time"
)

type rateError struct {
	rateLimited atomic.Bool
	enabled     bool
	state       atomic.Pointer[rateErrorState]
	errorCount  atomic.Int32
	threshold   int
	duration    time.Duration
}

type rateErrorState struct {
	timestamp time.Time
	err       error
}

func (r *rateError) isRateLimited() bool {
	if !r.enabled {
		return false
	}

	if r.rateLimited.Load() {
		return true
	}

	r.errorCount.Store(0) // reset the error count
	return false
}

// canDisableRateLimit checks if we can disable the limiter in the exporter.
func (r *rateError) canDisableRateLimit() bool {
	return time.Since(r.state.Load().timestamp) > r.duration
}

func (r *rateError) enableRateLimit(err error) {
	r.errorCount.Add(1)
	if r.errorCount.Load() < int32(r.threshold) {
		return
	}

	now := time.Now()
	r.state.Store(&rateErrorState{
		timestamp: now,
		err:       err,
	})
	r.rateLimited.Store(true)
}

func (r *rateError) disableRateLimit() {
	r.errorCount.Store(0)
	r.rateLimited.Store(false)
}

func (r *rateError) GetError() error {
	return r.state.Load().err
}
