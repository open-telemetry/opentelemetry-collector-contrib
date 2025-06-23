// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"errors"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
)

type rateError struct {
	rateLimited   atomic.Bool
	enabled       bool
	timestamp     atomic.Pointer[time.Time]
	internalError atomic.Pointer[error]
	errorCount    atomic.Int32
	threshold     int
	duration      time.Duration
}

func (r *rateError) isRateLimited() bool {
	if !r.enabled {
		return false
	}

	if r.rateLimited.Load() {
		return true
	}

	return false
}

// canDisableRateLimit checks if we can disable the limiter in the exporter.
func (r *rateError) canDisableRateLimit() bool {
	return time.Since(*r.timestamp.Load()) > r.duration
}

func (r *rateError) enableRateLimit() {
	r.errorCount.Add(1)
	if r.errorCount.Load() < int32(r.threshold) {
		return
	}

	now := time.Now()
	r.timestamp.Store(&now)
	r.rateLimited.Store(true)
	err := consumererror.NewPermanent(errors.New("rate limit exceeded at " + now.Format(time.RFC3339)))
	r.internalError.Store(&err)
}

func (r *rateError) disableRateLimit() {
	r.errorCount.Store(0)
	r.rateLimited.Store(false)
}

func (r *rateError) GetError() error {
	return *r.internalError.Load()
}
