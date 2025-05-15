// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"strconv"
	"sync/atomic"
	"time"
)

var (
	coralogixExporterRateLimitThreshold = "CORALOGIX_EXPORTER_RATE_LIMIT_THRESHOLD"
	coralogixExporterRateLimitDuration  = "CORALOGIX_EXPORTER_RATE_LIMIT_DURATION"
	rateLimitThreshold                  = getRateLimitThreshold()
	rateLimitDuration                   = getRateLimitDuration()
)

type rateError struct {
	rateLimited atomic.Bool
	state       atomic.Pointer[rateErrorState]
	errorCount  atomic.Int64
}

type rateErrorState struct {
	timestamp time.Time
	err       error
}

func (r *rateError) isRateLimited() bool {
	if enableRateLimiterFeatureGate.IsEnabled() && r.rateLimited.Load() {
		return true
	}
	r.errorCount.Store(0) // reset the error count
	return false
}

func (r *rateError) canDisableRateLimit() bool {
	if r.state.Load() == nil {
		return false
	}
	return time.Since(r.state.Load().timestamp) > rateLimitDuration
}

func (r *rateError) enableRateLimit(err error) {
	r.errorCount.Add(1)
	if r.errorCount.Load() < rateLimitThreshold {
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

func getRateLimitThreshold() int64 {
	threshold, err := strconv.ParseInt(coralogixExporterRateLimitThreshold, 10, 64)
	if err != nil {
		return 10
	}
	return threshold
}

func getRateLimitDuration() time.Duration {
	minDuration := 1 * time.Minute
	duration, err := time.ParseDuration(coralogixExporterRateLimitDuration)
	if err != nil {
		return minDuration
	}

	if duration < minDuration {
		return minDuration
	}

	return duration
}
