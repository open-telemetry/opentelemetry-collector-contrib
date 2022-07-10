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

// nolint:errcheck
package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupByTraceProcessor"

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/common"
)

// groupByTraceProcessor is a processor that keeps logs in memory for a given duration, with the expectation
// that the log will be complete once this duration expires. After the duration, the log is sent to the next consumer.
// This processor uses a buffered event machine, which converts operations into events for non-blocking processing, but
// keeping all operations serialized per worker scope. This ensures that we don't need locks but that the state is consistent across go routines.
// Initially, all incoming batches are split into different logs and distributed among workers by a hash of traceID in eventMachine.consume method.
// Afterwards, the log is registered with a go routine, which will be called after the given duration and dispatched to the event
// machine for further processing.
// The typical data flow looks like this:
// ConsumeLogs -> eventMachine.consume(log) -> event(logReceived) -> onLogReceived -> AfterFunc(duration, event(logExpired)) -> onTraceExpired
// async markAsReleased -> event(logReleased) -> onLogReleased -> nextConsumer
// Each worker in the eventMachine also uses a ring buffer to hold the in-flight log IDs, so that we don't hold more than the given maximum number
// of logs in memory/storage. Items that are evicted from the buffer are discarded without warning.
type groupByTraceProcessor struct {
	nextConsumer consumer.Logs
	config       common.Config
	logger       *zap.Logger

	// the event machine handling all operations for this processor
	eventMachine *eventMachine

	// the log storage
	st Storage
}

var _ component.LogsProcessor = (*groupByTraceProcessor)(nil)

const bufferSize = 10_000

// NewGroupByTraceProcessor returns a new processor.
func NewGroupByTraceProcessor(logger *zap.Logger, st Storage, nextConsumer consumer.Logs, config common.Config) *groupByTraceProcessor {
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
	eventMachine.onLogReceived = sp.onLogReceived
	eventMachine.onLogExpired = sp.onTraceExpired
	eventMachine.onLogReleased = sp.onLogReleased
	eventMachine.onLogRemoved = sp.onLogRemoved

	return sp
}

func (sp *groupByTraceProcessor) ConsumeLogs(_ context.Context, td plog.Logs) error {
	var errs error
	for _, singleTrace := range batchpersignal.SplitLogs(td) {
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
	stats.Record(context.Background(), mLogsEvicted.M(0))
	stats.Record(context.Background(), mIncompleteReleases.M(0))
	stats.Record(context.Background(), mNumLogsConf.M(int64(sp.config.NumTraces)))

	sp.eventMachine.startInBackground()
	return sp.st.start()
}

// Shutdown is invoked during service shutdown.
func (sp *groupByTraceProcessor) Shutdown(_ context.Context) error {
	sp.eventMachine.shutdown()
	return sp.st.shutdown()
}

func (sp *groupByTraceProcessor) onLogReceived(log logsWithID, worker *eventMachineWorker) error {
	traceID := log.id
	if worker.buffer.Contains(traceID) {
		sp.logger.Debug("log is already in memory storage")

		// it exists in memory already, just append the logRecords to the log in the storage
		if err := sp.addLogRecords(traceID, log.td); err != nil {
			return fmt.Errorf("couldn't add logRecords to existing log: %w", err)
		}

		// we are done with this log, move on
		return nil
	}

	// at this point, we determined that we haven't seen the log yet, so, record the
	// traceID in the map and the logRecords to the storage

	// place the log ID in the buffer, and check if an item had to be evicted
	evicted := worker.buffer.Put(traceID)
	if !evicted.IsEmpty() {
		// delete from the storage
		worker.fire(event{
			typ:     logRemoved,
			payload: evicted,
		})

		stats.Record(context.Background(), mLogsEvicted.M(1))

		sp.logger.Info("log evicted: in order to avoid this in the future, adjust the wait duration and/or number of logs to keep in memory",
			zap.String("traceID", evicted.HexString()))
	}

	// we have the traceID in the memory, place the logRecords in the storage too
	if err := sp.addLogRecords(traceID, log.td); err != nil {
		return fmt.Errorf("couldn't add logRecords to existing log: %w", err)
	}

	sp.logger.Debug("scheduled to release log", zap.Duration("duration", sp.config.WaitDuration))

	time.AfterFunc(sp.config.WaitDuration, func() {
		// if the event machine has stopped, it will just discard the event
		worker.fire(event{
			typ:     logExpired,
			payload: traceID,
		})
	})
	return nil
}

func (sp *groupByTraceProcessor) onTraceExpired(traceID pcommon.TraceID, worker *eventMachineWorker) error {
	sp.logger.Debug("processing expired", zap.String("traceID",
		traceID.HexString()))

	if !worker.buffer.Contains(traceID) {
		// we likely received multiple batches with logRecords for the same log
		// and released this log already
		sp.logger.Debug("skipping the processing of expired log",
			zap.String("traceID", traceID.HexString()))

		stats.Record(context.Background(), mIncompleteReleases.M(1))
		return nil
	}

	// delete from the map and erase its memory entry
	worker.buffer.Delete(traceID)

	// this might block, but we don't need to wait
	sp.logger.Debug("marking the log as released",
		zap.String("traceID", traceID.HexString()))
    go func() {
        _ = sp.markAsReleased(traceID, worker.fire)
    }()

	return nil
}

func (sp *groupByTraceProcessor) markAsReleased(traceID pcommon.TraceID, fire func(...event)) error {
	// #get is a potentially blocking operation
	log, err := sp.st.get(traceID)
	if err != nil {
		return fmt.Errorf("couldn't retrieve log %q from the storage: %w", traceID, err)
	}

	if log == nil {
		return fmt.Errorf("the log %q couldn't be found at the storage", traceID)
	}

	// signal that the log is ready to be released
	sp.logger.Debug("log marked as released", zap.String("traceID", traceID.HexString()))

	// atomically fire the two events, so that a concurrent shutdown won't leave
	// an orphaned log in the storage
	fire(event{
		typ:     logReleased,
		payload: log,
	}, event{
		typ:     logRemoved,
		payload: traceID,
	})
	return nil
}

func (sp *groupByTraceProcessor) onLogReleased(rss []plog.ResourceLogs) error {
	log := plog.NewLogs()
	for _, rs := range rss {
		trs := log.ResourceLogs().AppendEmpty()
		rs.CopyTo(trs)
	}
	stats.Record(context.Background(),
		mReleasedLogRecords.M(int64(log.LogRecordCount())),
		mReleasedLogs.M(1),
	)

	// Do async consuming not to block event worker
	go func() {
		if err := sp.nextConsumer.ConsumeLogs(context.Background(), log); err != nil {
			sp.logger.Error("consume failed", zap.Error(err))
		}
	}()
	return nil
}

func (sp *groupByTraceProcessor) onLogRemoved(traceID pcommon.TraceID) error {
	log, err := sp.st.delete(traceID)
	if err != nil {
		return fmt.Errorf("couldn't delete log %q from the storage: %w", traceID.HexString(), err)
	}

	if log == nil {
		return fmt.Errorf("log %q not found at the storage", traceID.HexString())
	}

	return nil
}

func (sp *groupByTraceProcessor) addLogRecords(traceID pcommon.TraceID, log plog.Logs) error {
	sp.logger.Debug("creating log at the storage", zap.String("traceID", traceID.HexString()))
	return sp.st.createOrAppend(traceID, log)
}
