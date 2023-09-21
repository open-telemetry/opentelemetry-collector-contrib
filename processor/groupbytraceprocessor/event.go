// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"context"
	"errors"
	"fmt"
	"hash/maphash"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const (
	// traces received from the previous processors
	traceReceived eventType = iota

	// traceID to be released
	traceExpired

	// released traces
	traceReleased

	// traceID to be removed
	traceRemoved
)

var (
	errNoTraceID = errors.New("trace doesn't have traceID")

	seed = maphash.MakeSeed()

	hashPool = sync.Pool{
		New: func() interface{} {
			var hash maphash.Hash
			hash.SetSeed(seed)
			return &hash
		},
	}

	eventTagKey = tag.MustNewKey("event")
)

type eventType int
type event struct {
	typ     eventType
	payload interface{}
}

type tracesWithID struct {
	id pcommon.TraceID
	td ptrace.Traces
}

// eventMachine is a machine that accepts events in a typically non-blocking manner,
// processing the events serially per worker scope, to ensure that data at the consumer is consistent.
// Just like the machine itself is non-blocking, consumers are expected to also not block
// on the callbacks, otherwise, events might pile up. When enough events are piled up, firing an
// event will block until enough capacity is available to accept the events.
type eventMachine struct {
	workers                   []*eventMachineWorker
	close                     chan struct{}
	metricsCollectionInterval time.Duration
	shutdownTimeout           time.Duration

	logger *zap.Logger

	onTraceReceived func(td tracesWithID, worker *eventMachineWorker) error
	onTraceExpired  func(traceID pcommon.TraceID, worker *eventMachineWorker) error
	onTraceReleased func(rss []ptrace.ResourceSpans) error
	onTraceRemoved  func(traceID pcommon.TraceID) error

	onError func(event)

	// shutdown sync
	shutdownLock *sync.RWMutex
	closed       bool
}

func newEventMachine(logger *zap.Logger, bufferSize int, numWorkers int, numTraces int) *eventMachine {
	em := &eventMachine{
		logger:                    logger,
		workers:                   make([]*eventMachineWorker, numWorkers),
		close:                     make(chan struct{}),
		shutdownLock:              &sync.RWMutex{},
		metricsCollectionInterval: time.Second,
		shutdownTimeout:           10 * time.Second,
	}
	for i := range em.workers {
		em.workers[i] = &eventMachineWorker{
			machine: em,
			buffer:  newRingBuffer(numTraces / numWorkers),
			events:  make(chan event, bufferSize/numWorkers),
		}
	}
	return em
}

func (em *eventMachine) startInBackground() {
	em.startWorkers()
	go em.periodicMetrics()
}

func (em *eventMachine) numEvents() int {
	var result int
	for _, worker := range em.workers {
		result += len(worker.events)
	}
	return result
}

func (em *eventMachine) periodicMetrics() {
	numEvents := em.numEvents()
	em.logger.Debug("recording current state of the queue", zap.Int("num-events", numEvents))
	stats.Record(context.Background(), mNumEventsInQueue.M(int64(numEvents)))

	em.shutdownLock.RLock()
	closed := em.closed
	em.shutdownLock.RUnlock()
	if closed {
		return
	}

	time.AfterFunc(em.metricsCollectionInterval, func() {
		em.periodicMetrics()
	})
}

func (em *eventMachine) startWorkers() {
	for _, worker := range em.workers {
		go worker.start()
	}
}

func (em *eventMachine) handleEvent(e event, w *eventMachineWorker) {
	switch e.typ {
	case traceReceived:
		if em.onTraceReceived == nil {
			em.logger.Debug("onTraceReceived not set, skipping event")
			em.callOnError(e)
			return
		}
		payload, ok := e.payload.(tracesWithID)
		if !ok {
			// the payload had an unexpected type!
			em.callOnError(e)
			return
		}

		em.handleEventWithObservability("onTraceReceived", func() error {
			return em.onTraceReceived(payload, w)
		})
	case traceExpired:
		if em.onTraceExpired == nil {
			em.logger.Debug("onTraceExpired not set, skipping event")
			em.callOnError(e)
			return
		}
		payload, ok := e.payload.(pcommon.TraceID)
		if !ok {
			// the payload had an unexpected type!
			em.callOnError(e)
			return
		}

		em.handleEventWithObservability("onTraceExpired", func() error {
			return em.onTraceExpired(payload, w)
		})
	case traceReleased:
		if em.onTraceReleased == nil {
			em.logger.Debug("onTraceReleased not set, skipping event")
			em.callOnError(e)
			return
		}
		payload, ok := e.payload.([]ptrace.ResourceSpans)
		if !ok {
			// the payload had an unexpected type!
			em.callOnError(e)
			return
		}

		em.handleEventWithObservability("onTraceReleased", func() error {
			return em.onTraceReleased(payload)
		})
	case traceRemoved:
		if em.onTraceRemoved == nil {
			em.logger.Debug("onTraceRemoved not set, skipping event")
			em.callOnError(e)
			return
		}
		payload, ok := e.payload.(pcommon.TraceID)
		if !ok {
			// the payload had an unexpected type!
			em.callOnError(e)
			return
		}

		em.handleEventWithObservability("onTraceRemoved", func() error {
			return em.onTraceRemoved(payload)
		})
	default:
		em.logger.Info("unknown event type", zap.Any("event", e.typ))
		em.callOnError(e)
		return
	}
}

// consume takes a single trace and routes it to one of the workers.
func (em *eventMachine) consume(td ptrace.Traces) error {
	traceID, err := getTraceID(td)
	if err != nil {
		return fmt.Errorf("eventmachine consume failed: %w", err)
	}

	var bucket uint64
	if len(em.workers) != 1 {
		bucket = workerIndexForTraceID(traceID, len(em.workers))
	}

	em.logger.Debug("scheduled trace to worker", zap.Uint64("id", bucket))

	em.workers[bucket].fire(event{
		typ:     traceReceived,
		payload: tracesWithID{id: traceID, td: td},
	})
	return nil
}

func workerIndexForTraceID(traceID pcommon.TraceID, numWorkers int) uint64 {
	hash := hashPool.Get().(*maphash.Hash)
	defer func() {
		hash.Reset()
		hashPool.Put(hash)
	}()

	_, _ = hash.Write(traceID[:])
	return hash.Sum64() % uint64(numWorkers)
}

func (em *eventMachine) shutdown() {
	em.logger.Info("shutting down the event manager", zap.Int("pending-events", em.numEvents()))
	em.shutdownLock.Lock()
	em.closed = true
	em.shutdownLock.Unlock()

	done := make(chan struct{})

	// we never return an error here
	ok, _ := doWithTimeout(em.shutdownTimeout, func() error {
		for {
			if em.numEvents() == 0 {
				return nil
			}
			time.Sleep(100 * time.Millisecond)

			// Do not leak goroutine
			select {
			case <-done:
				return nil
			default:
			}
		}
	})
	close(done)

	if !ok {
		em.logger.Info("forcing the shutdown of the event manager", zap.Int("pending-events", em.numEvents()))
	}
	close(em.close)
}

func (em *eventMachine) callOnError(e event) {
	if em.onError != nil {
		em.onError(e)
	}
}

// handleEventWithObservability uses the given function to process and event,
// recording the event's latency and timing out if it doesn't finish within a reasonable duration
func (em *eventMachine) handleEventWithObservability(event string, do func() error) {
	start := time.Now()
	succeeded, err := doWithTimeout(time.Second, do)
	duration := time.Since(start)

	_ = stats.RecordWithTags(context.Background(), []tag.Mutator{tag.Upsert(eventTagKey, event)}, mEventLatency.M(duration.Milliseconds()))

	if err != nil {
		em.logger.Error("failed to process event", zap.Error(err), zap.String("event", event))
	}
	if succeeded {
		em.logger.Debug("event finished", zap.String("event", event))
	} else {
		em.logger.Debug("event aborted", zap.String("event", event))
	}
}

type eventMachineWorker struct {
	machine *eventMachine

	// the ring buffer holds the IDs for all the in-flight traces
	buffer *ringBuffer

	events chan event
}

func (w *eventMachineWorker) start() {
	for {
		select {
		case e := <-w.events:
			w.machine.handleEvent(e, w)
		case <-w.machine.close:
			return
		}
	}
}

func (w *eventMachineWorker) fire(events ...event) {
	w.machine.shutdownLock.RLock()
	defer w.machine.shutdownLock.RUnlock()

	// we are not accepting new events
	if w.machine.closed {
		return
	}

	for _, e := range events {
		w.events <- e
	}
}

// doWithTimeout wraps a function in a timeout, returning whether it succeeded before timing out.
// If the function returns an error within the timeout, it's considered as succeeded and the error will be returned back to the caller.
func doWithTimeout(timeout time.Duration, do func() error) (bool, error) {
	done := make(chan error, 1)
	go func() {
		done <- do()
	}()

	select {
	case <-time.After(timeout):
		return false, nil
	case err := <-done:
		return true, err
	}
}

func getTraceID(td ptrace.Traces) (pcommon.TraceID, error) {
	rss := td.ResourceSpans()
	if rss.Len() == 0 {
		return pcommon.NewTraceIDEmpty(), errNoTraceID
	}

	ilss := rss.At(0).ScopeSpans()
	if ilss.Len() == 0 {
		return pcommon.NewTraceIDEmpty(), errNoTraceID
	}

	spans := ilss.At(0).Spans()
	if spans.Len() == 0 {
		return pcommon.NewTraceIDEmpty(), errNoTraceID
	}

	return spans.At(0).TraceID(), nil
}
