// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/cespare/xxhash/v2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// Attributes names for first and last observed timestamps
const (
	firstObservedTSAttr = "first_observed_timestamp"
	lastObservedTSAttr  = "last_observed_timestamp"
)

type hashedKey [8]byte

// timeNow can be reassigned for testing
var timeNow = time.Now

// logAggregator tracks the number of times a specific logRecord has been seen.
type logAggregator struct {
	resources         map[hashedKey]*resourceAggregator
	logCountAttribute string
	timezone          *time.Location
}

// newLogAggregator creates a new LogCounter.
func newLogAggregator(logCountAttribute string, timezone *time.Location) *logAggregator {
	return &logAggregator{
		resources:         make(map[hashedKey]*resourceAggregator),
		logCountAttribute: logCountAttribute,
		timezone:          timezone,
	}
}

// Export exports the counter as a Logs
func (l *logAggregator) Export() plog.Logs {
	logs := plog.NewLogs()

	for _, resourceAggregator := range l.resources {
		rl := logs.ResourceLogs().AppendEmpty()
		resourceAggregator.resource.CopyTo(rl.Resource())

		for _, scopeAggregator := range resourceAggregator.scopeCounters {
			sl := rl.ScopeLogs().AppendEmpty()
			scopeAggregator.scope.CopyTo(sl.Scope())

			for _, logAggregator := range scopeAggregator.logCounters {
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
	l.resources = make(map[hashedKey]*resourceAggregator)
}

// resourceAggregator dimensions the counter by resource.
type resourceAggregator struct {
	resource      pcommon.Resource
	scopeCounters map[hashedKey]*scopeAggregator
}

// newResourceAggregator creates a new ResourceCounter.
func newResourceAggregator(resource pcommon.Resource) *resourceAggregator {
	return &resourceAggregator{
		resource:      resource,
		scopeCounters: make(map[hashedKey]*scopeAggregator),
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
	logCounters map[hashedKey]*logCounter
}

// newScopeAggregator creates a new ScopeCounter.
func newScopeAggregator(scope pcommon.InstrumentationScope) *scopeAggregator {
	return &scopeAggregator{
		scope:       scope,
		logCounters: make(map[hashedKey]*logCounter),
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
/* #nosec G104 -- According to Hash interface write can never return an error */
func getResourceKey(resource pcommon.Resource) hashedKey {
	hasher := xxhash.New()
	attrsHash := pdatautil.MapHash(resource.Attributes())
	hasher.Write(attrsHash[:])
	hash := hasher.Sum(nil)

	// convert from slice to fixed size array to use as key
	var key hashedKey
	copy(key[:], hash)
	return key
}

// getScopeKey creates a unique hash for the scope to use as a map key
/* #nosec G104 -- According to Hash interface write can never return an error */
func getScopeKey(scope pcommon.InstrumentationScope) hashedKey {
	hasher := xxhash.New()
	attrsHash := pdatautil.MapHash(scope.Attributes())
	hasher.Write(attrsHash[:])
	hasher.Write([]byte(scope.Name()))
	hasher.Write([]byte(scope.Version()))
	hash := hasher.Sum(nil)

	// convert from slice to fixed size array to use as key
	var key hashedKey
	copy(key[:], hash)
	return key
}

// getLogKey creates a unique hash for the log record to use as a map key
/* #nosec G104 -- According to Hash interface write can never return an error */
func getLogKey(logRecord plog.LogRecord) hashedKey {
	hasher := xxhash.New()
	attrsHash := pdatautil.MapHash(logRecord.Attributes())
	hasher.Write(attrsHash[:])
	bodyHash := pdatautil.ValueHash(logRecord.Body())
	hasher.Write(bodyHash[:])
	hasher.Write([]byte(logRecord.SeverityNumber().String()))
	hasher.Write([]byte(logRecord.SeverityText()))
	hash := hasher.Sum(nil)

	// convert from slice to fixed size array to use as key
	var key hashedKey
	copy(key[:], hash)
	return key
}
