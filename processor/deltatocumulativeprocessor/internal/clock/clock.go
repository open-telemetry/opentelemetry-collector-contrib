// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clock // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/clock"

import (
	"sync"
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
	clock := &fake{}
	clock.Set(time.Time{})
	return clock
}

type fake struct {
	mtx sync.RWMutex
	ts  time.Time
}

func (f *fake) Set(now time.Time) {
	f.mtx.Lock()
	f.ts = now
	f.mtx.Unlock()
}

func (f *fake) Now() time.Time {
	f.mtx.RLock()
	defer f.mtx.RUnlock()
	return f.ts
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
