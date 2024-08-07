// Copyright  observIQ, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logdeduplicationprocessor

import (
	"hash/fnv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
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
	resources         map[[16]byte]*resourceAggregator
	logCountAttribute string
	timezone          *time.Location
}

// newLogAggregator creates a new LogCounter.
func newLogAggregator(logCountAttribute string, timezone *time.Location) *logAggregator {

	return &logAggregator{
		resources:         make(map[[16]byte]*resourceAggregator),
		logCountAttribute: logCountAttribute,
		timezone:          timezone,
	}
}

// Export exports the counter as a Logs
func (l *logAggregator) Export() plog.Logs {
	logs := plog.NewLogs()

	for _, resource := range l.resources {
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		resourceAttrs := resourceLogs.Resource().Attributes()
		resourceAttrs.EnsureCapacity(resourceAttrs.Len())
		resource.attributes.CopyTo(resourceAttrs)

		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		for _, lc := range resource.logCounters {
			lr := scopeLogs.LogRecords().AppendEmpty()

			baseRecord := lc.logRecord

			// Copy contents of base record
			baseRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(lc.firstObservedTimestamp))
			baseRecord.Body().CopyTo(lr.Body())

			lr.Attributes().EnsureCapacity(baseRecord.Attributes().Len())
			baseRecord.Attributes().CopyTo(lr.Attributes())

			lr.SetSeverityNumber(baseRecord.SeverityNumber())
			lr.SetSeverityText(baseRecord.SeverityText())

			// Add attributes for log count and timestamps
			lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(lc.firstObservedTimestamp))
			lr.SetTimestamp(pcommon.NewTimestampFromTime(timeNow()))
			lr.Attributes().PutInt(l.logCountAttribute, lc.count)

			firstTimestampStr := lc.firstObservedTimestamp.In(l.timezone).Format(time.RFC3339)
			lastTimestampStr := lc.lastObservedTimestamp.In(l.timezone).Format(time.RFC3339)
			lr.Attributes().PutStr(firstObservedTSAttr, firstTimestampStr)
			lr.Attributes().PutStr(lastObservedTSAttr, lastTimestampStr)
		}
	}

	return logs
}

// Add adds the logRecord to the resource aggregator that is identified by the resource attributes
func (l *logAggregator) Add(resourceAttrs pcommon.Map, logRecord plog.LogRecord) {
	key := pdatautil.MapHash(resourceAttrs)
	resourceCounter, ok := l.resources[key]
	if !ok {
		resourceCounter = newResourceAggregator(resourceAttrs)
		l.resources[key] = resourceCounter
	}

	resourceCounter.Add(logRecord)
}

// Reset resets the counter.
func (l *logAggregator) Reset() {
	l.resources = make(map[[16]byte]*resourceAggregator)
}

// resourceAggregator dimensions the counter by resource.
type resourceAggregator struct {
	attributes  pcommon.Map
	logCounters map[[8]byte]*logCounter
}

// newResourceAggregator creates a new ResourceCounter.
func newResourceAggregator(attributes pcommon.Map) *resourceAggregator {
	return &resourceAggregator{
		attributes:  attributes,
		logCounters: make(map[[8]byte]*logCounter),
	}
}

// Add increments the counter that the logRecord matches.
func (r *resourceAggregator) Add(logRecord plog.LogRecord) {
	key := getLogKey(logRecord)
	lc, ok := r.logCounters[key]
	if !ok {
		lc = newLogCounter(logRecord)
		lc.firstObservedTimestamp = timeNow().UTC()
		r.logCounters[key] = lc
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
		logRecord: logRecord,
		count:     0,
	}
}

// Increment increments the counter.
func (a *logCounter) Increment() {
	a.lastObservedTimestamp = timeNow().UTC()
	a.count++
}

// getLogKey creates a unique md5 hash for the log record to use as a map key
/* #nosec G104 -- According to Hash interface write can never return an error */
func getLogKey(logRecord plog.LogRecord) [8]byte {
	hasher := fnv.New64()
	attrHash := pdatautil.MapHash(logRecord.Attributes())

	hasher.Write(attrHash[:])
	bodyHash := pdatautil.ValueHash(logRecord.Body())
	hasher.Write(bodyHash[:])
	hasher.Write([]byte(logRecord.SeverityNumber().String()))
	hasher.Write([]byte(logRecord.SeverityText()))
	hash := hasher.Sum(nil)

	// convert from slice to fixed size array to use as key
	var key [8]byte
	copy(key[:], hash)
	return key
}
