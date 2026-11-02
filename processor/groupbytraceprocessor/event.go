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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
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

	// Subtrace event. Only used when EmitStrategy == EmitStrategyService:

	// the worker's subtrace timer has gone off. The event carries no payload:
	// what is due is whatever the worker's own deadlines say is due when it gets
	// here, which is the only account of it that can't be out of date.
	subtraceTick
)

var (
	errNoTraceID = errors.New("trace doesn't have traceID")

	seed = maphash.MakeSeed()

	hashPool = sync.Pool{
		New: func() any {
			var hash maphash.Hash
			hash.SetSeed(seed)
			return &hash
		},
	}
)

type (
	eventType int
	event     struct {
		typ     eventType
		payload any
	}
)

func (t eventType) String() string {
	switch t {
	case traceReceived:
		return "onTraceReceived"
	case traceExpired:
		return "onTraceExpired"
	case traceReleased:
		return "onTraceReleased"
	case traceRemoved:
		return "onTraceRemoved"
	case subtraceTick:
		return "subtrace_tick"
	}
	return "unknown"
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
	eventTimeout              time.Duration

	logger          *zap.Logger
	telemetry       *metadata.TelemetryBuilder
	onTraceReceived func(td tracesWithID, worker *eventMachineWorker) error
	onTraceExpired  func(traceID pcommon.TraceID, worker *eventMachineWorker) error
	onTraceReleased func(rss []ptrace.ResourceSpans) error
	onTraceRemoved  func(traceID pcommon.TraceID) error

	onSubtraceTick func(worker *eventMachineWorker) error

	onError func(event)

	// shutdown sync
	shutdownLock *sync.RWMutex
	closed       bool

	// workersWG tracks the running worker goroutines, so that shutdown can wait
	// for a handler that is still in progress instead of returning while it is
	// touching worker state or starting asynchronous work.
	workersWG sync.WaitGroup
}

func newEventMachine(logger *zap.Logger, bufferSize, numWorkers, numTraces int, telemetry *metadata.TelemetryBuilder) *eventMachine {
	em := &eventMachine{
		logger:                    logger,
		telemetry:                 telemetry,
		workers:                   make([]*eventMachineWorker, numWorkers),
		close:                     make(chan struct{}),
		shutdownLock:              &sync.RWMutex{},
		metricsCollectionInterval: time.Second,
		shutdownTimeout:           10 * time.Second,
		eventTimeout:              time.Second,
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
	em.telemetry.ProcessorGroupbytraceNumEventsInQueue.Record(context.Background(), int64(numEvents))

	if em.onSubtraceTick != nil {
		var numSubtraces int
		for _, w := range em.workers {
			if w.subSt != nil {
				numSubtraces += w.subSt.count()
			}
		}
		em.telemetry.ProcessorGroupbytraceNumTracesInMemory.Record(context.Background(), int64(numSubtraces))
	}

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
		em.workersWG.Go(worker.start)
	}
}

func (em *eventMachine) handleEvent(e event, w *eventMachineWorker) {
	switch e.typ {
	case traceReceived:
		if em.onTraceReceived == nil {
			em.logger.Debug("event callback not set, skipping event", zap.Stringer("event", e.typ))
			em.callOnError(e)
			return
		}
		payload, ok := e.payload.(tracesWithID)
		if !ok {
			// the payload had an unexpected type!
			em.callOnError(e)
			return
		}

		em.handleEventWithObservability(e.typ, func() error {
			return em.onTraceReceived(payload, w)
		})
	case traceExpired:
		if em.onTraceExpired == nil {
			em.logger.Debug("event callback not set, skipping event", zap.Stringer("event", e.typ))
			em.callOnError(e)
			return
		}
		payload, ok := e.payload.(pcommon.TraceID)
		if !ok {
			// the payload had an unexpected type!
			em.callOnError(e)
			return
		}

		em.handleEventWithObservability(e.typ, func() error {
			return em.onTraceExpired(payload, w)
		})
	case traceReleased:
		if em.onTraceReleased == nil {
			em.logger.Debug("event callback not set, skipping event", zap.Stringer("event", e.typ))
			em.callOnError(e)
			return
		}
		payload, ok := e.payload.([]ptrace.ResourceSpans)
		if !ok {
			// the payload had an unexpected type!
			em.callOnError(e)
			return
		}

		em.handleEventWithObservability(e.typ, func() error {
			return em.onTraceReleased(payload)
		})
	case traceRemoved:
		if em.onTraceRemoved == nil {
			em.logger.Debug("event callback not set, skipping event", zap.Stringer("event", e.typ))
			em.callOnError(e)
			return
		}
		payload, ok := e.payload.(pcommon.TraceID)
		if !ok {
			// the payload had an unexpected type!
			em.callOnError(e)
			return
		}

		em.handleEventWithObservability(e.typ, func() error {
			return em.onTraceRemoved(payload)
		})
	case subtraceTick:
		if em.onSubtraceTick == nil {
			em.logger.Debug("event callback not set, skipping event", zap.Stringer("event", e.typ))
			em.callOnError(e)
			return
		}
		em.handleEventWithObservability(e.typ, func() error {
			return em.onSubtraceTick(w)
		})
	default:
		em.logger.Info("unknown event type", zap.Stringer("event", e.typ))
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
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		// Check immediately first
		if em.numEvents() == 0 {
			return nil
		}

		for {
			select {
			case <-done:
				return nil
			case <-ticker.C:
				if em.numEvents() == 0 {
					return nil
				}
			}
		}
	})
	close(done)

	if !ok {
		em.logger.Info("forcing the shutdown of the event manager", zap.Int("pending-events", em.numEvents()))
	}
	close(em.close)

	// Returning while a handler is still running would let it start work the
	// caller has no way left to wait for. A handler is abandoned after a second,
	// so this waits at most that long.
	em.workersWG.Wait()
}

func (em *eventMachine) callOnError(e event) {
	if em.onError != nil {
		em.onError(e)
	}
}

// handleEventWithObservability uses the given function to process and event,
// recording the event's latency and timing out if it doesn't finish within a reasonable duration
func (em *eventMachine) handleEventWithObservability(typ eventType, do func() error) {
	name := typ.String()
	start := time.Now()
	var succeeded bool
	var err error

	// subtraceTick has a bounded completion time and is not meant to be called concurrently;
	// long-running calls should complete instead of being cancelled.
	if typ != subtraceTick {
		succeeded, err = doWithTimeout(em.eventTimeout, do)
	} else {
		err = do()
		succeeded = true
	}
	duration := time.Since(start)
	em.telemetry.ProcessorGroupbytraceEventLatency.Record(context.Background(), duration.Milliseconds(), metric.WithAttributeSet(attribute.NewSet(attribute.String("event", name))))

	if err != nil {
		em.logger.Error("failed to process event", zap.Error(err), zap.String("event", name))
	}
	if succeeded {
		em.logger.Debug("event finished", zap.String("event", name))
	} else {
		em.logger.Debug("event aborted", zap.String("event", name))
	}
}

type eventMachineWorker struct {
	machine *eventMachine

	// buffer holds the IDs for all in-flight traces (EmitStrategyTrace).
	buffer *ringBuffer

	// subtraceBuffer holds the IDs for all in-flight subtraces (EmitStrategyService).
	subtraceBuffer *subtraceRingBuffer

	// deadlines says when each buffered subtrace next falls due, and
	// subtraceTimer wakes the worker for the earliest of them. Both belong to
	// the worker and are only ever touched from a worker turn.
	deadlines     *subtraceDeadlines
	subtraceTimer *time.Timer

	// subSt holds the spans buffered for this worker's subtraces
	// (EmitStrategyService). Traces are routed to a worker by trace ID, so a
	// worker is the only one to touch its own storage, and workers do not
	// contend with each other for it.
	subSt subtraceStorage

	events chan event
}

// armSubtraceTimer points the worker's timer at the earliest deadline it holds.
// Only a worker turn should call it.
func (w *eventMachineWorker) armSubtraceTimer() {
	next, held := w.deadlines.next()
	if !held {
		w.stopSubtraceTimer()
		return
	}

	wait := max(time.Until(next), 0)
	if w.subtraceTimer == nil {
		w.subtraceTimer = time.AfterFunc(wait, func() {
			// if the event machine has stopped, it will just discard the event
			w.fire(event{typ: subtraceTick})
		})
		return
	}
	w.subtraceTimer.Reset(wait)
}

func (w *eventMachineWorker) stopSubtraceTimer() {
	if w.subtraceTimer != nil {
		w.subtraceTimer.Stop()
	}
}

func (w *eventMachineWorker) start() {
	for {
		// Prioritize shutdown: check if we should stop before processing next event
		select {
		case <-w.machine.close:
			return
		default:
		}

		// Process events or handle shutdown
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
