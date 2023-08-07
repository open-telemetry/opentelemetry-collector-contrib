// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"time"
)

// TimeProvider allows to get current Unix second
type TimeProvider interface {
	getCurSecond() int64
}

// MonotonicClock provides monotonic real clock-based current Unix second.
// Use it when creating a NewComposite which should measure sample rates
// against a realtime clock (this is almost always what you want to do,
// the exception is usually only automated testing where you may want
// to have fake clocks).
type MonotonicClock struct{}

func (c MonotonicClock) getCurSecond() int64 {
	return time.Now().Unix()
}
