// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clock // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/clock"

import (
	"time"
)

var clock Clock = Real{}

func Change(c Clock) {
	clock = c
}

type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

func Now() time.Time {
	return clock.Now()
}

func After(d time.Duration) <-chan time.Time {
	return clock.After(d)
}

func Until(t time.Time) time.Duration {
	return t.Sub(Now())
}

func Sleep(d time.Duration) {
	<-After(d)
}

type Real struct{}

func (r Real) Now() time.Time {
	return time.Now()
}

func (r Real) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

type Settable interface {
	Clock
	Set(now time.Time)
}

func Fake() Settable {
	return &fake{sig: make(chan time.Time)}
}

type fake struct {
	time time.Time

	sig chan time.Time
}

func (f *fake) Set(now time.Time) {
	f.time = now

	select {
	case f.sig <- now:
	default:
	}
}

func (f fake) Now() time.Time {
	return f.time
}

func (f *fake) After(d time.Duration) <-chan time.Time {
	var (
		end  = f.Now().Add(d)
		done = make(chan time.Time)
		wait = make(chan struct{})
	)

	go func() {
		close(wait)
		for f.Now().Before(end) {
			time.Sleep(time.Millisecond / 10)
		}
		done <- f.Now()
		close(done)
	}()
	<-wait

	return done
}
