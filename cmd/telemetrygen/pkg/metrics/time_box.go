// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

/*
TimeBox allows for only one metric timeseries per second. It'll generate additional timeseries if the attribute is
requested within a one-second window
*/
type TimeBox struct {
	// The attribute to use for the timebox
	Attribute string

	// locks attribute creation for a moment once a second
	mutex *sync.Mutex

	// The attribute value last used
	offset int64
}

func NewTimeBox(attribute string) *TimeBox {
	tb := &TimeBox{
		Attribute: attribute,
		offset:    0,
		mutex:     &sync.Mutex{},
	}

	go tb.resetTimerLoop(time.NewTimer(time.Second))
	return tb
}

func (tb *TimeBox) GetAttribute() attribute.KeyValue {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	tb.offset++
	return attribute.Int(tb.Attribute, int(tb.offset))
}

// resetTimer resets the timer to 1 second every time the timer is stopped.
func (tb *TimeBox) resetTimerLoop(t *time.Timer) func() {
	for {
		<-t.C
		tb.mutex.Lock()
		tb.offset = 0
		t.Reset(time.Second)
		tb.mutex.Unlock()
	}
}
