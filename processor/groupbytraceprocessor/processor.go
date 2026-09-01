// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"context"
	"fmt"
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

	// subtrace storage (used when EmitStrategy == EmitStrategyService)
	subSt subtraceStorage
}

var _ processor.Traces = (*groupByTraceProcessor)(nil)

const bufferSize = 10_000

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
	if sp.subSt != nil {
		return sp.subSt.start()
	}
	return sp.st.start()
}

// Shutdown is invoked during service shutdown.
func (sp *groupByTraceProcessor) Shutdown(ctx context.Context) error {
	sp.eventMachine.shutdown()

	if sp.subSt != nil {
		// Flush remaining orphan spans that were never claimed by a subtrace timer.
		tids := sp.subSt.traceIDs()
		for _, tid := range tids {
			members, _ := sp.subSt.getRemainder(tid)
			_ = sp.subSt.deleteTrace(tid)
			if len(members) > 0 {
				if err := sp.nextConsumer.ConsumeTraces(ctx, assemble(members)); err != nil {
					sp.logger.Error("shutdown drain consume failed", zap.Error(err))
				}
			}
		}
		return sp.subSt.shutdown()
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
	traceID := trace.id

	// Insert all spans from this batch into the span-level index.
	rss := trace.td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				if err := sp.subSt.insertSpan(traceID, rs.Resource(), ss.Scope(), ss.Spans().At(k)); err != nil {
					return fmt.Errorf("couldn't insert span: %w", err)
				}
			}
		}
	}

	// Discover local roots in the now-updated index.
	// No trace-level timer is started; orphan spans are handled by ring-buffer
	// overflow (dropped) and shutdown drain.
	roots := sp.subSt.localRoots(traceID)
	for _, rootSpanID := range roots {
		id := subtraceID{traceID: traceID, spanID: rootSpanID}
		if worker.subtraceBuffer.contains(id) {
			continue // already tracking this subtrace
		}
		// Register in ring buffer; handle eviction.
		if evicted, ok := worker.subtraceBuffer.put(id); ok {
			sp.telemetryBuilder.ProcessorGroupbytraceTracesEvicted.Add(context.Background(), 1)
			worker.fire(event{typ: subtraceRemoved, payload: evicted})
		}
		// Start the per-subtrace wait timer.
		capturedID := id
		sp.logger.Debug("scheduled to release subtrace", zap.Duration("duration", sp.config.WaitDuration))
		time.AfterFunc(sp.config.WaitDuration, func() {
			worker.fire(event{typ: subtraceExpired, payload: capturedID})
		})
	}
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

func (sp *groupByTraceProcessor) onSubtraceExpired(id subtraceID, worker *eventMachineWorker) error {
	if !worker.subtraceBuffer.contains(id) {
		sp.telemetryBuilder.ProcessorGroupbytraceIncompleteReleases.Add(context.Background(), 1)
		return nil
	}
	worker.subtraceBuffer.delete(id)
	go func() {
		_ = sp.markSubtraceAsReleased(id, worker.fire)
	}()
	return nil
}

func (sp *groupByTraceProcessor) markSubtraceAsReleased(id subtraceID, fire func(...event)) error {
	members, err := sp.subSt.getSubtrace(id.traceID, id.spanID)
	if err != nil {
		return fmt.Errorf("couldn't retrieve subtrace: %w", err)
	}
	if len(members) == 0 {
		return nil
	}
	assembled := assemble(members)
	fire(
		event{typ: subtraceReleased, payload: assembled},
		event{typ: subtraceRemoved, payload: id},
	)
	return nil
}

func (sp *groupByTraceProcessor) onSubtraceReleased(td ptrace.Traces) error {
	sp.telemetryBuilder.ProcessorGroupbytraceSpansReleased.Add(context.Background(), int64(td.SpanCount()))
	sp.telemetryBuilder.ProcessorGroupbytraceTracesReleased.Add(context.Background(), 1)
	go func() {
		if err := sp.nextConsumer.ConsumeTraces(context.Background(), td); err != nil {
			sp.logger.Error("consume failed", zap.Error(err))
		}
	}()
	return nil
}

func (sp *groupByTraceProcessor) onSubtraceRemoved(id subtraceID) error {
	_, err := sp.subSt.deleteSubtrace(id.traceID, id.spanID)
	return err
}
