// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

/*
timeBox allows for only one metric timeseries per second. It'll generate additional timeseries if the attribute is
requested within a one-second window
*/
type timeBox struct {
	// enforceUnique is set to true if the attribute should be unique
	// for each second. If false, the attribute will always be 0.
	enforceUnique bool

	// locks attribute creation for a moment once a second
	mutex *sync.Mutex

	// used to gracefully shut down the timer
	stop chan struct{}

	// The attribute value last used
	offset int64
}

const timeBoxAttributeName = "timebox"

func newTimeBox(enforceUnique bool, uniqueTimeLimit time.Duration) *timeBox {
	tb := &timeBox{
		offset:        0,
		enforceUnique: enforceUnique,
		mutex:         &sync.Mutex{},
		stop:          make(chan struct{}),
	}

	go tb.resetTimerLoop(time.NewTimer(uniqueTimeLimit))
	return tb
}

func (tb *timeBox) getAttribute() attribute.KeyValue {
	if !tb.enforceUnique {
		return attribute.Int(timeBoxAttributeName, 0)
	}
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	tb.offset++
	return attribute.Int(timeBoxAttributeName, int(tb.offset))
}

// resetTimer resets the timer to 1 second every time the timer is stopped.
func (tb *timeBox) resetTimerLoop(t *time.Timer) {
	// no-op when enforceUnique is false
	if !tb.enforceUnique {
		return
	}
	for {
		select {
		case <-tb.stop:
			t.Stop()
			return
		case <-t.C:
			tb.mutex.Lock()
			tb.offset = 0
			t.Reset(time.Second)
			tb.mutex.Unlock()
		}
	}
}

func (tb *timeBox) shutdown() {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	if tb.stop != nil {
		close(tb.stop)
	}
}
