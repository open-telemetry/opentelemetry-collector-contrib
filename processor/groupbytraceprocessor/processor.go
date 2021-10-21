// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package groupbytraceprocessor

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
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
	nextConsumer consumer.Traces
	config       Config
	logger       *zap.Logger

	// the event machine handling all operations for this processor
	eventMachine *eventMachine

	// the trace storage
	st storage
}

var _ component.TracesProcessor = (*groupByTraceProcessor)(nil)

const bufferSize = 10_000

// newGroupByTraceProcessor returns a new processor.
func newGroupByTraceProcessor(logger *zap.Logger, st storage, nextConsumer consumer.Traces, config Config) *groupByTraceProcessor {
	// the event machine will buffer up to N concurrent events before blocking
	eventMachine := newEventMachine(logger, 10000, config.NumWorkers, config.NumTraces)

	sp := &groupByTraceProcessor{
		logger:       logger,
		nextConsumer: nextConsumer,
		config:       config,
		eventMachine: eventMachine,
		st:           st,
	}

	// register the callbacks
	eventMachine.onTraceReceived = sp.onTraceReceived
	eventMachine.onTraceExpired = sp.onTraceExpired
	eventMachine.onTraceReleased = sp.onTraceReleased
	eventMachine.onTraceRemoved = sp.onTraceRemoved

	return sp
}

func (sp *groupByTraceProcessor) ConsumeTraces(_ context.Context, td pdata.Traces) error {
	var errs error
	for _, singleTrace := range batchpersignal.SplitTraces(td) {
		errs = multierr.Append(errs, sp.eventMachine.consume(singleTrace))
	}
	return errs
}

func (sp *groupByTraceProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start is invoked during service startup.
func (sp *groupByTraceProcessor) Start(context.Context, component.Host) error {
	// start these metrics, as it might take a while for them to receive their first event
	stats.Record(context.Background(), mTracesEvicted.M(0))
	stats.Record(context.Background(), mIncompleteReleases.M(0))
	stats.Record(context.Background(), mNumTracesConf.M(int64(sp.config.NumTraces)))

	sp.eventMachine.startInBackground()
	return sp.st.start()
}

// Shutdown is invoked during service shutdown.
func (sp *groupByTraceProcessor) Shutdown(_ context.Context) error {
	sp.eventMachine.shutdown()
	return sp.st.shutdown()
}

func (sp *groupByTraceProcessor) onTraceReceived(trace tracesWithID, worker *eventMachineWorker) error {
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

		stats.Record(context.Background(), mTracesEvicted.M(1))

		sp.logger.Info("trace evicted: in order to avoid this in the future, adjust the wait duration and/or number of traces to keep in memory",
			zap.String("traceID", evicted.HexString()))
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

func (sp *groupByTraceProcessor) onTraceExpired(traceID pdata.TraceID, worker *eventMachineWorker) error {
	sp.logger.Debug("processing expired", zap.String("traceID",
		traceID.HexString()))

	if !worker.buffer.contains(traceID) {
		// we likely received multiple batches with spans for the same trace
		// and released this trace already
		sp.logger.Debug("skipping the processing of expired trace",
			zap.String("traceID", traceID.HexString()))

		stats.Record(context.Background(), mIncompleteReleases.M(1))
		return nil
	}

	// delete from the map and erase its memory entry
	worker.buffer.delete(traceID)

	// this might block, but we don't need to wait
	sp.logger.Debug("marking the trace as released",
		zap.String("traceID", traceID.HexString()))
	go sp.markAsReleased(traceID, worker.fire)

	return nil
}

func (sp *groupByTraceProcessor) markAsReleased(traceID pdata.TraceID, fire func(...event)) error {
	// #get is a potentially blocking operation
	trace, err := sp.st.get(traceID)
	if err != nil {
		return fmt.Errorf("couldn't retrieve trace %q from the storage: %w", traceID, err)
	}

	if trace == nil {
		return fmt.Errorf("the trace %q couldn't be found at the storage", traceID)
	}

	// signal that the trace is ready to be released
	sp.logger.Debug("trace marked as released", zap.String("traceID", traceID.HexString()))

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

func (sp *groupByTraceProcessor) onTraceReleased(rss []pdata.ResourceSpans) error {
	trace := pdata.NewTraces()
	for _, rs := range rss {
		trs := trace.ResourceSpans().AppendEmpty()
		rs.CopyTo(trs)
	}
	stats.Record(context.Background(),
		mReleasedSpans.M(int64(trace.SpanCount())),
		mReleasedTraces.M(1),
	)

	// Do async consuming not to block event worker
	go func() {
		if err := sp.nextConsumer.ConsumeTraces(context.Background(), trace); err != nil {
			sp.logger.Error("consume failed", zap.Error(err))
		}
	}()
	return nil
}

func (sp *groupByTraceProcessor) onTraceRemoved(traceID pdata.TraceID) error {
	trace, err := sp.st.delete(traceID)
	if err != nil {
		return fmt.Errorf("couldn't delete trace %q from the storage: %w", traceID.HexString(), err)
	}

	if trace == nil {
		return fmt.Errorf("trace %q not found at the storage", traceID.HexString())
	}

	return nil
}

func (sp *groupByTraceProcessor) addSpans(traceID pdata.TraceID, trace pdata.Traces) error {
	sp.logger.Debug("creating trace at the storage", zap.String("traceID", traceID.HexString()))
	return sp.st.createOrAppend(traceID, trace)
}
