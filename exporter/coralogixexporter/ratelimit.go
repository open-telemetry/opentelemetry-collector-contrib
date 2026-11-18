// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
)

type rateError struct {
	rateLimited   atomic.Bool
	enabled       bool
	timestamp     atomic.Pointer[time.Time]
	resumeAt      atomic.Pointer[time.Time]
	internalError atomic.Pointer[error]
	status        atomic.Pointer[string]
	reason        atomic.Int32
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

// enableRateLimit increments the consecutive permanent-error counter. When the
// counter reaches threshold it opens the drop window, stores a reason-specific
// permanent error, and returns true so the caller can log once.
func (r *rateError) enableRateLimit(reason dropReason, status string) bool {
	r.errorCount.Add(1)
	if r.errorCount.Load() < int32(r.threshold) {
		return false
	}

	now := time.Now()
	resumeAt := now.Add(r.duration)
	r.timestamp.Store(&now)
	r.resumeAt.Store(&resumeAt)
	r.reason.Store(int32(reason))
	r.status.Store(&status)
	opened := !r.rateLimited.Swap(true)
	err := dropWindowError(reason, resumeAt, r.threshold)
	r.internalError.Store(&err)
	return opened
}

// disableRateLimit clears the drop window. It returns true when an open window
// was actually closed so the caller can log once.
func (r *rateError) disableRateLimit() bool {
	wasLimited := r.rateLimited.Swap(false)
	r.errorCount.Store(0)
	return wasLimited
}

func (r *rateError) GetError() error {
	return *r.internalError.Load()
}

func (r *rateError) getReason() dropReason {
	return dropReason(r.reason.Load())
}

func (r *rateError) getResumeAt() time.Time {
	if p := r.resumeAt.Load(); p != nil {
		return *p
	}
	return time.Time{}
}

func dropWindowError(reason dropReason, resumeAt time.Time, threshold int) error {
	resume := resumeAt.Format(time.RFC3339)
	var msg string
	switch reason {
	case dropReasonAuth:
		msg = fmt.Sprintf("authentication failed (private key rotated or invalid); dropping data until %s after %d consecutive failures", resume, threshold)
	case dropReasonQuota:
		msg = fmt.Sprintf("quota exceeded; dropping data until %s after %d consecutive failures", resume, threshold)
	default:
		msg = fmt.Sprintf("rate limit exceeded; dropping data until %s after %d consecutive failures", resume, threshold)
	}
	return consumererror.NewPermanent(errors.New(msg))
}
