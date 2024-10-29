// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor/internal/metadata"
)

// Attributes names for first and last observed timestamps
const (
	firstObservedTSAttr = "first_observed_timestamp"
	lastObservedTSAttr  = "last_observed_timestamp"
)

// timeNow can be reassigned for testing
var timeNow = time.Now

// logAggregator tracks the number of times a specific logRecord has been seen.
type logAggregator struct {
	resources         map[uint64]*resourceAggregator
	logCountAttribute string
	timezone          *time.Location
	telemetryBuilder  *metadata.TelemetryBuilder
}

// newLogAggregator creates a new LogCounter.
func newLogAggregator(logCountAttribute string, timezone *time.Location, telemetryBuilder *metadata.TelemetryBuilder) *logAggregator {
	return &logAggregator{
		resources:         make(map[uint64]*resourceAggregator),
		logCountAttribute: logCountAttribute,
		timezone:          timezone,
		telemetryBuilder:  telemetryBuilder,
	}
}

// Export exports the counter as a Logs
func (l *logAggregator) Export(ctx context.Context) plog.Logs {
	logs := plog.NewLogs()

	for _, resourceAggregator := range l.resources {
		rl := logs.ResourceLogs().AppendEmpty()
		resourceAggregator.resource.CopyTo(rl.Resource())

		for _, scopeAggregator := range resourceAggregator.scopeCounters {
			sl := rl.ScopeLogs().AppendEmpty()
			scopeAggregator.scope.CopyTo(sl.Scope())

			for _, logAggregator := range scopeAggregator.logCounters {
				// Record aggregated logs records
				l.telemetryBuilder.DedupProcessorAggregatedLogs.Record(ctx, logAggregator.count)

				lr := sl.LogRecords().AppendEmpty()
				logAggregator.logRecord.CopyTo(lr)

				// Set log record timestamps
				lr.SetTimestamp(pcommon.NewTimestampFromTime(timeNow()))
				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(logAggregator.firstObservedTimestamp))

				// Add attributes for log count and first/last observed timestamps
				lr.Attributes().EnsureCapacity(lr.Attributes().Len() + 3)
				lr.Attributes().PutInt(l.logCountAttribute, logAggregator.count)
				firstTimestampStr := logAggregator.firstObservedTimestamp.In(l.timezone).Format(time.RFC3339)
				lr.Attributes().PutStr(firstObservedTSAttr, firstTimestampStr)
				lastTimestampStr := logAggregator.lastObservedTimestamp.In(l.timezone).Format(time.RFC3339)
				lr.Attributes().PutStr(lastObservedTSAttr, lastTimestampStr)
			}
		}
	}

	return logs
}

// Add adds the logRecord to the resource aggregator that is identified by the resource attributes
func (l *logAggregator) Add(resource pcommon.Resource, scope pcommon.InstrumentationScope, logRecord plog.LogRecord) {
	key := getResourceKey(resource)
	resourceAggregator, ok := l.resources[key]
	if !ok {
		resourceAggregator = newResourceAggregator(resource)
		l.resources[key] = resourceAggregator
	}
	resourceAggregator.Add(scope, logRecord)
}

// Reset resets the counter.
func (l *logAggregator) Reset() {
	l.resources = make(map[uint64]*resourceAggregator)
}

// resourceAggregator dimensions the counter by resource.
type resourceAggregator struct {
	resource      pcommon.Resource
	scopeCounters map[uint64]*scopeAggregator
}

// newResourceAggregator creates a new ResourceCounter.
func newResourceAggregator(resource pcommon.Resource) *resourceAggregator {
	return &resourceAggregator{
		resource:      resource,
		scopeCounters: make(map[uint64]*scopeAggregator),
	}
}

// Add increments the counter that the logRecord matches.
func (r *resourceAggregator) Add(scope pcommon.InstrumentationScope, logRecord plog.LogRecord) {
	key := getScopeKey(scope)
	scopeAggregator, ok := r.scopeCounters[key]
	if !ok {
		scopeAggregator = newScopeAggregator(scope)
		r.scopeCounters[key] = scopeAggregator
	}
	scopeAggregator.Add(logRecord)
}

// scopeAggregator dimensions the counter by scope.
type scopeAggregator struct {
	scope       pcommon.InstrumentationScope
	logCounters map[uint64]*logCounter
}

// newScopeAggregator creates a new ScopeCounter.
func newScopeAggregator(scope pcommon.InstrumentationScope) *scopeAggregator {
	return &scopeAggregator{
		scope:       scope,
		logCounters: make(map[uint64]*logCounter),
	}
}

// Add increments the counter that the logRecord matches.
func (s *scopeAggregator) Add(logRecord plog.LogRecord) {
	key := getLogKey(logRecord)
	lc, ok := s.logCounters[key]
	if !ok {
		lc = newLogCounter(logRecord)
		s.logCounters[key] = lc
	}
	lc.Increment()
}

// logCounter is a counter for a log record.
type logCounter struct {
	logRecord              plog.LogRecord
	firstObservedTimestamp time.Time
	lastObservedTimestamp  time.Time
	count                  int64
}

// newLogCounter creates a new AttributeCounter.
func newLogCounter(logRecord plog.LogRecord) *logCounter {
	return &logCounter{
		logRecord:              logRecord,
		count:                  0,
		firstObservedTimestamp: timeNow().UTC(),
		lastObservedTimestamp:  timeNow().UTC(),
	}
}

// Increment increments the counter.
func (a *logCounter) Increment() {
	a.lastObservedTimestamp = timeNow().UTC()
	a.count++
}

// getResourceKey creates a unique hash for the resource to use as a map key
func getResourceKey(resource pcommon.Resource) uint64 {
	return pdatautil.Hash64(
		pdatautil.WithMap(resource.Attributes()),
	)
}

// getScopeKey creates a unique hash for the scope to use as a map key
func getScopeKey(scope pcommon.InstrumentationScope) uint64 {
	return pdatautil.Hash64(
		pdatautil.WithMap(scope.Attributes()),
		pdatautil.WithString(scope.Name()),
		pdatautil.WithString(scope.Version()),
	)
}

// getLogKey creates a unique hash for the log record to use as a map key
func getLogKey(logRecord plog.LogRecord) uint64 {
	return pdatautil.Hash64(
		pdatautil.WithMap(logRecord.Attributes()),
		pdatautil.WithValue(logRecord.Body()),
		pdatautil.WithString(logRecord.SeverityNumber().String()),
		pdatautil.WithString(logRecord.SeverityText()),
	)
}
