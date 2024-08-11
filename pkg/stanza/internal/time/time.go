// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package time // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/time"

import (
	"time"

	"github.com/jonboulle/clockwork"
)

var Now = time.Now
var Since = time.Since

// Clock where Now() always returns a greater value than the previous return value
type AlwaysIncreasingClock struct {
	clockwork.FakeClock
}

func NewAlwaysIncreasingClock() AlwaysIncreasingClock {
	return AlwaysIncreasingClock{
		FakeClock: clockwork.NewFakeClock(),
	}
}

func (c AlwaysIncreasingClock) Now() time.Time {
	c.FakeClock.Advance(time.Nanosecond)
	return c.FakeClock.Now()
}

func (c AlwaysIncreasingClock) Since(t time.Time) time.Duration {
	// ensure that internal c.FakeClock.Now() will return a greater value
	c.FakeClock.Advance(time.Nanosecond)
	return c.FakeClock.Since(t)
}
