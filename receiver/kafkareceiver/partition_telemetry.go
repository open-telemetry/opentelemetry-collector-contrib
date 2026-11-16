// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// partitionTelemetry tracks one assignment without retaining its worker or
// mailbox. Its cumulative wait survives reassignment.
type partitionTelemetry struct {
	attrs          attribute.Set
	decodeOptions  [1]metric.RecordOption
	consumeOptions [1]metric.RecordOption
	mu             sync.Mutex
	waiting        bool
	active         int64
	stopped        bool
	recovering     bool
	waitStart      time.Time
	dataWait       time.Duration
}

// newPartitionTelemetry creates telemetry with cached stage attributes.
func newPartitionTelemetry(attrs attribute.Set) *partitionTelemetry {
	stageOptions := func(stage string) [1]metric.RecordOption {
		values := attrs.ToSlice()
		values = append(values, attribute.String("stage", stage))
		return [1]metric.RecordOption{metric.WithAttributeSet(attribute.NewSet(values...))}
	}
	return &partitionTelemetry{
		attrs:          attrs,
		decodeOptions:  stageOptions("decode"),
		consumeOptions: stageOptions("consume"),
	}
}

// setWaiting updates whether the worker is waiting for data.
func (p *partitionTelemetry) setWaiting(waiting bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.waiting = waiting
	p.updateWait()
}

// startCall starts tracking a record-processing call.
func (p *partitionTelemetry) startCall() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active++
	p.updateWait()
}

// finishCall completes a call and enters recovery on failure.
func (p *partitionTelemetry) finishCall(err error) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active--
	// A failed cross-fetch call can finish while the worker awaits data.
	// Keep recovery separate from waiting until the rewind has completed.
	if err != nil {
		p.recovering = true
	}
	p.updateWait()
}

// resume clears recovery after a rewind.
func (p *partitionTelemetry) resume() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.recovering = false
	p.updateWait()
}

// stop permanently stops data-wait tracking.
func (p *partitionTelemetry) stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
	p.updateWait()
}

// updateWait runs under mu. A cross-fetch worker can be waiting for another
// batch while earlier calls run; only the last call's return starts data wait.
func (p *partitionTelemetry) updateWait() {
	waiting := p.waiting && p.active == 0 && !p.stopped && !p.recovering
	if waiting && p.waitStart.IsZero() {
		p.waitStart = time.Now()
	} else if !waiting && !p.waitStart.IsZero() {
		p.dataWait += time.Since(p.waitStart)
		p.waitStart = time.Time{}
	}
}

// snapshot returns active calls and total data wait, including the ongoing wait.
func (p *partitionTelemetry) snapshot() (int64, time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	wait := p.dataWait
	if !p.waitStart.IsZero() {
		wait += time.Since(p.waitStart)
	}
	return p.active, wait
}

// observePartitionDataWait reports cumulative wait for all tracked partitions.
func (c *franzConsumer) observePartitionDataWait(_ context.Context, observer metric.Float64Observer) error {
	if partitions := c.partitionTelemetrySnapshot.Load(); partitions != nil {
		for _, p := range *partitions {
			_, wait := p.snapshot()
			// Keep frozen totals observable so delta readers retain their baseline.
			observer.Observe(wait.Seconds(), metric.WithAttributeSet(p.attrs))
		}
	}
	return nil
}

// observePartitionInFlight reports active calls for current assignments.
func (c *franzConsumer) observePartitionInFlight(_ context.Context, observer metric.Int64Observer) error {
	if assignments := c.assignmentsSnapshot.Load(); assignments != nil {
		for _, p := range *assignments {
			if p.telemetry != nil && p.ctx.Err() == nil {
				active, _ := p.telemetry.snapshot()
				observer.Observe(active, metric.WithAttributeSet(p.attrs))
			}
		}
	}
	return nil
}
