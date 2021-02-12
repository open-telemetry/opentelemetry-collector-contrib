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

package groupbyauthprocessor

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// groupByAuthProcessor is a processor that keeps spans grouped for specificic authentication token for a given duration, with the expectation
// that the trace will be complete once this duration expires. After the duration, the trace is sent to the next consumer.
// This processor uses a buffered event machine, which converts operations into events for non-blocking processing, but
// keeping all operations serialized. This ensures that we don't need locks but that the state is consistent across go routines.
// Each in-flight trace is registered with a go routine, which will be called after the given duration and dispatched to the event
// machine for further processing.
// The typical data flow looks like this:
// ConsumeTraces -> event(traceReceived) -> onBatchReceived -> AfterFunc(duration, event(traceExpired)) -> onTraceExpired
// async markAsReleased -> event(traceReleased) -> onTraceReleased -> nextConsumer
// This processor uses also a ring buffer to hold the in-flight trace IDs, so that we don't hold more than the given maximum number
// of traces in memory/storage. Items that are evicted from the buffer are discarded without warning.
type groupByAuthProcessor struct {
	nextConsumer consumer.TracesConsumer
	config       Config
	logger       *zap.Logger

	// the event machine handling all operations for this processor
	eventMachine *eventMachine

	// the ring buffer holding the IDs for all the in-flight traces
	ringBuffer *ringBuffer

	// the trace storage
	st storage
}

var _ component.TracesProcessor = (*groupByAuthProcessor)(nil)

// newGroupByAuthProcessor returns a new processor.
func newGroupByAuthProcessor(logger *zap.Logger, st storage, nextConsumer consumer.TracesConsumer, config Config) *groupByAuthProcessor {
	// the event machine will buffer up to N concurrent events before blocking
	eventMachine := newEventMachine(logger, 10000)

	sp := &groupByAuthProcessor{
		logger:       logger,
		nextConsumer: nextConsumer,
		config:       config,
		eventMachine: eventMachine,
		ringBuffer:   newRingBuffer(config.NumTraces),
		st:           st,
	}

	// register the callbacks
	eventMachine.onBatchReceived = sp.onBatchReceived
	eventMachine.onTokenExpired = sp.onTokenExpired
	eventMachine.onBatchReleased = sp.onBatchReleased
	eventMachine.onTokenRemoved = sp.onTokenRemoved

	return sp
}

func (sp *groupByAuthProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	cc, ok := client.FromContext(ctx)
	if !ok || cc.Token == "" {
		stats.Record(context.Background(), mBatchesNoToken.M(1))
		return sp.nextConsumer.ConsumeTraces(ctx, td)
	}
	stats.Record(context.Background(), mBatchesWithToken.M(1))

	sp.eventMachine.fire(event{
		typ:     batchReceived,
		token:   cc.Token,
		payload: td,
	})
	return nil
}

func (sp *groupByAuthProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

// Start is invoked during service startup.
func (sp *groupByAuthProcessor) Start(context.Context, component.Host) error {
	// start these metrics, as it might take a while for them to receive their first event
	stats.Record(context.Background(), mTracesEvicted.M(0))
	stats.Record(context.Background(), mIncompleteReleases.M(0))
	stats.Record(context.Background(), mNumTracesConf.M(int64(sp.config.NumTraces)))

	sp.eventMachine.startInBackground()
	return sp.st.start()
}

// Shutdown is invoked during service shutdown.
func (sp *groupByAuthProcessor) Shutdown(_ context.Context) error {
	sp.eventMachine.shutdown()
	return sp.st.shutdown()
}

func (sp *groupByAuthProcessor) onBatchReceived(token string, batch pdata.Traces) error {
	if sp.ringBuffer.contains(token) {
		// it exists in memory already, just append the spans to the trace in the storage
		if err := sp.addBatch(token, batch); err != nil {
			return fmt.Errorf("couldn't add traces: %w", err)
		}

		// we are done with this trace, move on
		return nil
	}

	// at this point, we determined that we haven't seen the trace yet, so, record the
	// traceID in the map and the spans to the storage

	// place the trace ID in the buffer, and check if an item had to be evicted
	evicted := sp.ringBuffer.put(token)
	if evicted != "" {
		// delete from the storage
		sp.eventMachine.fire(event{
			typ:     tokenRemoved,
			payload: evicted,
		})

		stats.Record(context.Background(), mTracesEvicted.M(1))

		sp.logger.Info("token evicted: in order to avoid this in the future, adjust the wait duration and/or number of traces to keep in memory")
	}

	// we have the traceID in the memory, place the spans in the storage too
	if err := sp.addBatch(token, batch); err != nil {
		return fmt.Errorf("couldn't add traces: %w", err)
	}

	sp.logger.Debug("scheduled to release trace", zap.Duration("duration", sp.config.WaitDuration))

	time.AfterFunc(sp.config.WaitDuration, func() {
		// if the event machine has stopped, it will just discard the event
		sp.eventMachine.fire(event{
			typ:     tokenExpired,
			payload: token,
		})
	})

	return nil
}

func (sp *groupByAuthProcessor) onTokenExpired(token string) error {
	sp.logger.Debug("processing expired", zap.String("token", token))

	if !sp.ringBuffer.contains(token) {
		// we likely received multiple traces with matching token
		// and released such set
		sp.logger.Debug("skipping the processing of expired trace",
			zap.String("token", token))

		stats.Record(context.Background(), mIncompleteReleases.M(1))
		return nil
	}

	// delete from the map and erase its memory entry
	sp.ringBuffer.delete(token)

	// this might block, but we don't need to wait
	sp.logger.Debug("marking the trace as released", zap.String("token", token))
	go sp.markAsReleased(token)

	return nil
}

func (sp *groupByAuthProcessor) markAsReleased(token string) error {
	// #get is a potentially blocking operation
	traces, found := sp.st.get(token)
	if !found {
		return fmt.Errorf("couldn't retrieve traces for token %s", token)
	}

	// signal that the trace is ready to be released
	sp.logger.Debug("trace marked as released", zap.String("token", token))

	// atomically fire the two events, so that a concurrent shutdown won't leave
	// an orphaned trace in the storage
	sp.eventMachine.fire(event{
		typ:     batchReleased,
		token:   token,
		payload: traces,
	}, event{
		typ:     tokenRemoved,
		payload: token,
	})
	return nil
}

func (sp *groupByAuthProcessor) onBatchReleased(token string, traces pdata.Traces) error {
	c := client.Client{
		IP:    "",
		Token: token,
	}
	ctx := client.NewContext(context.Background(), &c)
	stats.Record(context.Background(), mReleasedSpans.M(int64(traces.SpanCount())))
	stats.Record(context.Background(), mReleasedTraces.M(1))
	return sp.nextConsumer.ConsumeTraces(ctx, traces)
}

func (sp *groupByAuthProcessor) onTokenRemoved(token string) error {
	_, err := sp.st.delete(token)
	if !err {
		return fmt.Errorf("couldn't delete traces for token %q", token)
	}

	return nil
}

func (sp *groupByAuthProcessor) addBatch(token string, traces pdata.Traces) error {
	return sp.st.createOrAppend(token, traces)
}
