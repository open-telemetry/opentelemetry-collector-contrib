// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

// groupByTraceProcessor is a processor that keeps traces in memory for a given duration, with the expectation
// that the trace will be complete once this duration expires. After the duration, the trace is sent to the next consumer.
// This processor uses a buffered event machine, which converts operations into events for non-blocking processing, but
// keeping all operations serialized per worker scope. This ensures that we don't need locks but that the state is consistent across go routines.
// Initially, all incoming batches are split into different traces and distributed among workers by a hash of traceID in eventMachine.consume method.
// Afterwards, the trace is registered with a go routine, which will be called after the given duration and dispatched to the event
// machine for further processing.
// The typical data flow looks like this:
// ConsumeTraces -> eventMachine.consume(trace) -> event(traceReceived) -> onTraceReceived -> AfterFunc(duration, event(traceExpired)) -> onTraceExpired
// async markAsReleased -> event(traceReleased) -> onTraceReleased -> nextConsumer
// Each worker in the eventMachine also uses a ring buffer to hold the in-flight trace IDs, so that we don't hold more than the given maximum number
// of traces in memory/storage. Items that are evicted from the buffer are discarded without warning.
type groupByTraceProcessor struct {
	nextConsumer     consumer.Traces
	config           Config
	logger           *zap.Logger
	telemetryBuilder *metadata.TelemetryBuilder
	// the event machine handling all operations for this processor
	eventMachine *eventMachine

	// trace storage (used when EmitStrategy == EmitStrategyTrace)
	st traceStorage

	// releases tracks the subtrace releases currently in flight. Their spans are
	// out of storage already, so Shutdown waits on this rather than leaving them
	// to a goroutine it has stopped being able to hear from.
	releases sync.WaitGroup
}

var _ processor.Traces = (*groupByTraceProcessor)(nil)

const bufferSize = 10_000

// releaseCoalesceFraction sets how closely a subtrace's successive releases may
// follow one another, as a fraction of wait_duration: a subtrace is woken at
// most once per wait_duration/releaseCoalesceFraction. It trades up to that much
// release latency for a bound on how often a subtrace re-divides its spans into
// calls. See nextSubtraceDeadline.
const releaseCoalesceFraction = 10

// newGroupByTraceProcessor returns a new processor.
func newGroupByTraceProcessor(set processor.Settings, nextConsumer consumer.Traces, config Config) *groupByTraceProcessor {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil
	}

	// the event machine will buffer up to N concurrent events before blocking
	eventMachine := newEventMachine(set.Logger, 10000, config.NumWorkers, config.NumTraces, telemetryBuilder)

	sp := &groupByTraceProcessor{
		logger:           set.Logger,
		nextConsumer:     nextConsumer,
		config:           config,
		telemetryBuilder: telemetryBuilder,
		eventMachine:     eventMachine,
	}

	// register the callbacks
	eventMachine.onTraceReceived = sp.onTraceReceived
	eventMachine.onTraceExpired = sp.onTraceExpired
	eventMachine.onTraceReleased = sp.onTraceReleased
	eventMachine.onTraceRemoved = sp.onTraceRemoved

	return sp
}

func (sp *groupByTraceProcessor) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	var errs error
	for _, singleTrace := range batchpersignal.SplitTraces(td) {
		errs = multierr.Append(errs, sp.eventMachine.consume(singleTrace))
	}
	return errs
}

func (*groupByTraceProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start is invoked during service startup.
func (sp *groupByTraceProcessor) Start(context.Context, component.Host) error {
	// start these metrics, as it might take a while for them to receive their first event
	sp.telemetryBuilder.ProcessorGroupbytraceTracesEvicted.Add(context.Background(), 0)
	sp.telemetryBuilder.ProcessorGroupbytraceIncompleteReleases.Add(context.Background(), 0)
	sp.telemetryBuilder.ProcessorGroupbytraceConfNumTraces.Record(context.Background(), int64(sp.config.NumTraces))
	sp.eventMachine.startInBackground()
	if sp.config.EmitStrategy == EmitStrategyService {
		var errs error
		for _, w := range sp.eventMachine.workers {
			errs = multierr.Append(errs, w.subSt.start())
		}
		return errs
	}
	return sp.st.start()
}

// Shutdown is invoked during service shutdown.
func (sp *groupByTraceProcessor) Shutdown(ctx context.Context) error {
	sp.eventMachine.shutdown()

	if sp.config.EmitStrategy == EmitStrategyService {
		// The event machine has returned its workers, so no further release can
		// start. Let the ones already under way finish: their spans left storage
		// before the machine stopped and the drain below can no longer see them.
		sp.releases.Wait()

		// Flush whatever is still buffered, rather than dropping it.
		var errs error
		for _, w := range sp.eventMachine.workers {
			w.stopSubtraceTimer()
			for _, id := range w.subSt.subtraceIDs() {
				calls, _ := w.subSt.deleteSubtrace(id)
				for _, call := range calls {
					sp.releaseSubtrace(ctx, assemble(call))
				}
			}
			errs = multierr.Append(errs, w.subSt.shutdown())
		}
		return errs
	}

	return sp.st.shutdown()
}

func (sp *groupByTraceProcessor) onTraceReceived(trace tracesWithID, worker *eventMachineWorker) error {
	if sp.config.EmitStrategy == EmitStrategyService {
		return sp.onTraceReceivedSubtrace(trace, worker)
	}

	traceID := trace.id
	if worker.buffer.contains(traceID) {
		sp.logger.Debug("trace is already in memory storage")

		// it exists in memory already, just append the spans to the trace in the storage
		if err := sp.addSpans(traceID, trace.td); err != nil {
			return fmt.Errorf("couldn't add spans to existing trace: %w", err)
		}

		// we are done with this trace, move on
		return nil
	}

	// at this point, we determined that we haven't seen the trace yet, so, record the
	// traceID in the map and the spans to the storage

	// place the trace ID in the buffer, and check if an item had to be evicted
	evicted := worker.buffer.put(traceID)
	if !evicted.IsEmpty() {
		// delete from the storage
		worker.fire(event{
			typ:     traceRemoved,
			payload: evicted,
		})
		sp.telemetryBuilder.ProcessorGroupbytraceTracesEvicted.Add(context.Background(), 1)

		sp.logger.Info("trace evicted: in order to avoid this in the future, adjust the wait duration and/or number of traces to keep in memory",
			zap.Stringer("traceID", evicted))
	}

	// we have the traceID in the memory, place the spans in the storage too
	if err := sp.addSpans(traceID, trace.td); err != nil {
		return fmt.Errorf("couldn't add spans to existing trace: %w", err)
	}

	sp.logger.Debug("scheduled to release trace", zap.Duration("duration", sp.config.WaitDuration))

	time.AfterFunc(sp.config.WaitDuration, func() {
		// if the event machine has stopped, it will just discard the event
		worker.fire(event{
			typ:     traceExpired,
			payload: traceID,
		})
	})
	return nil
}

func (sp *groupByTraceProcessor) onTraceReceivedSubtrace(trace tracesWithID, worker *eventMachineWorker) error {
	// Taking the batch in may add deadlines and, through eviction, drop them.
	// Re-arming once at the end covers every change this turn made.
	defer worker.armSubtraceTimer()

	// Only read from the clock once per-batch instead of for every span,
	// which helps keep deadlines for per-service calls aligned instead
	// of potentially splitting them across deadline-check ticks.
	arrivedAt := time.Now()

	var errs error
	for _, rs := range trace.td.ResourceSpans().All() {
		rctx := newResourceContext(rs)
		id := subtraceID{traceID: trace.id, serviceID: rctx.serviceID}

		for _, ss := range rs.ScopeSpans().All() {
			sctx := newSpanContext(rctx, ss.Scope(), ss.SchemaUrl())
			if err := worker.subSt.insertScopeSpans(id, sctx, ss, arrivedAt); err != nil {
				return multierr.Append(errs, fmt.Errorf("couldn't insert spans: %w", err))
			}
		}

		if worker.subtraceBuffer.contains(id) {
			continue // already waiting to be released
		}
		if evicted, ok := worker.subtraceBuffer.put(id); ok {
			errs = multierr.Append(errs, sp.evictSubtrace(evicted, worker))
		}
		worker.deadlines.set(id, arrivedAt.Add(sp.config.WaitDuration))
	}
	return errs
}

// evictSubtrace handles a subtrace pushed out of the ring buffer. Eviction
// bounds what the processor holds; the spans go to the next consumer rather
// than being lost.
func (sp *groupByTraceProcessor) evictSubtrace(id subtraceID, worker *eventMachineWorker) error {
	sp.telemetryBuilder.ProcessorGroupbytraceTracesEvicted.Add(context.Background(), 1)
	sp.logger.Info("subtrace evicted and released early: in order to avoid this in the future, adjust the wait duration and/or number of traces to keep in memory",
		zap.Stringer("traceID", id.traceID))

	worker.deadlines.remove(id)
	calls, err := worker.subSt.deleteSubtrace(id)
	if err != nil {
		return fmt.Errorf("couldn't delete subtrace: %w", err)
	}
	sp.releaseCalls(calls)
	return nil
}

func (sp *groupByTraceProcessor) onTraceExpired(traceID pcommon.TraceID, worker *eventMachineWorker) error {
	sp.logger.Debug("processing expired", zap.Stringer("traceID", traceID))

	if !worker.buffer.contains(traceID) {
		// we likely received multiple batches with spans for the same trace
		// and released this trace already
		sp.logger.Debug("skipping the processing of expired trace", zap.Stringer("traceID", traceID))
		sp.telemetryBuilder.ProcessorGroupbytraceIncompleteReleases.Add(context.Background(), 1)
		return nil
	}

	// delete from the map and erase its memory entry
	worker.buffer.delete(traceID)

	// this might block, but we don't need to wait
	sp.logger.Debug("marking the trace as released", zap.Stringer("traceID", traceID))
	go func() {
		_ = sp.markAsReleased(traceID, worker.fire)
	}()

	return nil
}

func (sp *groupByTraceProcessor) markAsReleased(traceID pcommon.TraceID, fire func(...event)) error {
	// #get is a potentially blocking operation
	trace, err := sp.st.get(traceID)
	if err != nil {
		return fmt.Errorf("couldn't retrieve trace %q from the storage: %w", traceID, err)
	}

	if trace == nil {
		return fmt.Errorf("the trace %q couldn't be found at the storage", traceID)
	}

	// signal that the trace is ready to be released
	sp.logger.Debug("trace marked as released", zap.Stringer("traceID", traceID))

	// atomically fire the two events, so that a concurrent shutdown won't leave
	// an orphaned trace in the storage
	fire(event{
		typ:     traceReleased,
		payload: trace,
	}, event{
		typ:     traceRemoved,
		payload: traceID,
	})
	return nil
}

func (sp *groupByTraceProcessor) onTraceReleased(rss []ptrace.ResourceSpans) error {
	trace := ptrace.NewTraces()
	for _, rs := range rss {
		trs := trace.ResourceSpans().AppendEmpty()
		rs.CopyTo(trs)
	}

	sp.telemetryBuilder.ProcessorGroupbytraceSpansReleased.Add(context.Background(), int64(trace.SpanCount()))
	sp.telemetryBuilder.ProcessorGroupbytraceTracesReleased.Add(context.Background(), 1)

	// Do async consuming not to block event worker
	go func() {
		if err := sp.nextConsumer.ConsumeTraces(context.Background(), trace); err != nil {
			sp.logger.Error("consume failed", zap.Error(err))
		}
	}()
	return nil
}

func (sp *groupByTraceProcessor) onTraceRemoved(traceID pcommon.TraceID) error {
	trace, err := sp.st.delete(traceID)
	if err != nil {
		return fmt.Errorf("couldn't delete trace %q from the storage: %w", traceID, err)
	}

	if trace == nil {
		return fmt.Errorf("trace %q not found at the storage", traceID)
	}

	return nil
}

func (sp *groupByTraceProcessor) addSpans(traceID pcommon.TraceID, trace ptrace.Traces) error {
	sp.logger.Debug("creating trace at the storage", zap.Stringer("traceID", traceID))
	return sp.st.createOrAppend(traceID, trace)
}

// onSubtraceTick releases every subtrace that has come due. The deadlines and
// the ring buffer are both the worker's own and are settled together here, so a
// subtrace is never released on the strength of a deadline it no longer holds.
func (sp *groupByTraceProcessor) onSubtraceTick(worker *eventMachineWorker) error {
	defer worker.armSubtraceTimer()

	now := time.Now()
	cutoff := now.Add(-sp.config.WaitDuration)

	var errs error
	for _, id := range worker.deadlines.popDue(now) {
		if !worker.subtraceBuffer.contains(id) {
			// A deadline is set and dropped with the ring buffer entry it belongs
			// to, so reaching this means the two have come apart.
			sp.telemetryBuilder.ProcessorGroupbytraceIncompleteReleases.Add(context.Background(), 1)
			sp.logger.Debug("subtrace came due with no ring buffer entry",
				zap.Stringer("traceID", id.traceID), zap.String("serviceID", fmt.Sprintf("%x", id.serviceID)))
			continue
		}

		// Only the calls that have waited out wait_duration go now. A trace that
		// came back to this service after the deadline was set has a later one of
		// its own, and keeps its place in the buffer until then.
		due, nextArrival, err := worker.subSt.releaseDue(id, cutoff)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("couldn't retrieve subtrace: %w", err))
			continue
		}

		if nextArrival.IsZero() {
			worker.subtraceBuffer.delete(id)
		} else {
			worker.deadlines.set(id, sp.nextSubtraceDeadline(now, nextArrival))
		}

		if len(due) == 0 {
			// The spans are already gone, released by an earlier tick.
			sp.logger.Debug("subtrace came due with no spans to release",
				zap.Stringer("traceID", id.traceID), zap.String("serviceID", fmt.Sprintf("%x", id.serviceID)))
			continue
		}

		sp.releaseCalls(due)
	}
	return errs
}

// nextSubtraceDeadline says when a subtrace that still holds undue calls should
// next be woken, given the release that has just run at now.
//
// Without the floor of releaseCoalesceFraction*wait_duration, a subtrace is
// woken once per distinct first arrival it holds and re-divides all its spans
// on each waking. The floor collapses near wakings into one batch.
func (sp *groupByTraceProcessor) nextSubtraceDeadline(now, nextArrival time.Time) time.Time {
	due := nextArrival.Add(sp.config.WaitDuration)
	if floor := now.Add(sp.config.WaitDuration / releaseCoalesceFraction); due.Before(floor) {
		return floor
	}
	return due
}

// releaseCalls assembles and emits calls off the worker goroutine, so a large
// subtrace doesn't block the events queued behind it.
//
// Batches go straight to the next consumer: storage hands spans over on return,
// so events dropped by a concurrent shutdown would lose them, and the drain in
// Shutdown can no longer see them.
func (sp *groupByTraceProcessor) releaseCalls(calls [][]*bufferedSpan) {
	if len(calls) == 0 {
		return
	}

	// Registering on the worker, ahead of the goroutine, is what lets Shutdown
	// treat every release the worker has decided on as in flight.
	sp.releases.Go(func() {
		for _, call := range calls {
			sp.releaseSubtrace(context.Background(), assemble(call))
		}
	})
}

// releaseSubtrace hands an assembled subtrace to the next consumer. It consumes
// synchronously: every caller is off the worker goroutine already.
func (sp *groupByTraceProcessor) releaseSubtrace(ctx context.Context, td ptrace.Traces) {
	sp.telemetryBuilder.ProcessorGroupbytraceSpansReleased.Add(context.Background(), int64(td.SpanCount()))
	sp.telemetryBuilder.ProcessorGroupbytraceTracesReleased.Add(context.Background(), 1)
	if err := sp.nextConsumer.ConsumeTraces(ctx, td); err != nil {
		sp.logger.Error("consume failed", zap.Error(err))
	}
}
