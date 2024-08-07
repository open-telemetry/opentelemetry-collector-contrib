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
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func Test_newLogAggregator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	aggregator := newLogAggregator(cfg.LogCountAttribute, time.UTC)
	require.Equal(t, cfg.LogCountAttribute, aggregator.logCountAttribute)
	require.Equal(t, time.UTC, aggregator.timezone)
	require.NotNil(t, aggregator.resources)
}

func Test_logAggregatorAdd(t *testing.T) {
	oldTimeNow := timeNow
	defer func() {
		timeNow = oldTimeNow
	}()

	// Set timeNow to return a known value
	firstExpectedTimestamp := time.Now().UTC()
	timeNow = func() time.Time {
		return firstExpectedTimestamp
	}

	// Setup aggregator
	aggregator := newLogAggregator("log_count", time.UTC)
	logRecord := plog.NewLogRecord()
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("one", "two")

	expectedResourceKey := pdatautil.MapHash(resourceAttrs)
	expectedLogKey := getLogKey(logRecord)

	// Add logRecord
	aggregator.Add(resourceAttrs, logRecord)

	// Check resourceCounter was set
	resourceCounter, ok := aggregator.resources[expectedResourceKey]
	require.True(t, ok)
	require.Equal(t, resourceAttrs, resourceCounter.attributes)

	// Check logCounter was set
	lc, ok := resourceCounter.logCounters[expectedLogKey]
	require.True(t, ok)

	// Check fields on logCounter
	require.Equal(t, logRecord, lc.logRecord)
	require.Equal(t, int64(1), lc.count)
	require.Equal(t, firstExpectedTimestamp, lc.firstObservedTimestamp)
	require.Equal(t, firstExpectedTimestamp, lc.lastObservedTimestamp)

	// Add a matching logRecord to update counter and last observedTimestamp
	secondExpectedTimestamp := time.Now().Add(2 * time.Minute).UTC()
	timeNow = func() time.Time {
		return secondExpectedTimestamp
	}

	aggregator.Add(resourceAttrs, logRecord)
	require.Equal(t, int64(2), lc.count)
	require.Equal(t, secondExpectedTimestamp, lc.lastObservedTimestamp)
}

func Test_logAggregatorReset(t *testing.T) {
	aggregator := newLogAggregator("log_count", time.UTC)
	for i := 0; i < 2; i++ {
		resourceAttrs := pcommon.NewMap()
		resourceAttrs.PutInt("i", int64(i))
		key := pdatautil.MapHash(resourceAttrs)
		aggregator.resources[key] = newResourceAggregator(resourceAttrs)
	}

	require.Len(t, aggregator.resources, 2)

	aggregator.Reset()

	require.Len(t, aggregator.resources, 0)
}

func Test_logAggregatorExport(t *testing.T) {
	oldTimeNow := timeNow
	defer func() {
		timeNow = oldTimeNow
	}()

	location, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	// Set timeNow to return a known value
	expectedTimestamp := time.Now().UTC()
	expectedTimestampStr := expectedTimestamp.In(location).Format(time.RFC3339)
	timeNow = func() time.Time {
		return expectedTimestamp
	}

	// Setup aggregator

	aggregator := newLogAggregator(defaultLogCountAttribute, location)
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("one", "two")
	expectedHash := pdatautil.MapHash(resourceAttrs)

	logRecord := generateTestLogRecord(t, "body string")

	// Add logRecord
	aggregator.Add(resourceAttrs, logRecord)

	exportedLogs := aggregator.Export()
	require.Equal(t, 1, exportedLogs.LogRecordCount())
	require.Equal(t, 1, exportedLogs.ResourceLogs().Len())

	// Check resource
	rl := exportedLogs.ResourceLogs().At(0)
	actualAttrs := rl.Resource().Attributes()
	actualHash := pdatautil.MapHash(actualAttrs)
	require.Equal(t, expectedHash, actualHash)

	require.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)

	require.Equal(t, 1, sl.LogRecords().Len())
	actualLogRecord := sl.LogRecords().At(0)

	// Check logRecord
	require.Equal(t, logRecord.Body().AsString(), actualLogRecord.Body().AsString())
	require.Equal(t, logRecord.SeverityNumber(), actualLogRecord.SeverityNumber())
	require.Equal(t, logRecord.SeverityText(), actualLogRecord.SeverityText())
	require.Equal(t, expectedTimestamp.UnixMilli(), actualLogRecord.ObservedTimestamp().AsTime().UnixMilli())
	require.Equal(t, expectedTimestamp.UnixMilli(), actualLogRecord.Timestamp().AsTime().UnixMilli())

	actualRawAttrs := actualLogRecord.Attributes().AsRaw()
	for key, val := range logRecord.Attributes().AsRaw() {
		actualVal, ok := actualRawAttrs[key]
		require.True(t, ok)
		require.Equal(t, val, actualVal)
	}

	// Ensure new attributes were added
	actualLogCount, ok := actualRawAttrs[defaultLogCountAttribute]
	require.True(t, ok)
	require.Equal(t, int64(1), actualLogCount)

	actualFirstObserved, ok := actualRawAttrs[firstObservedTSAttr]
	require.True(t, ok)
	require.Equal(t, expectedTimestampStr, actualFirstObserved)

	actualLastObserved, ok := actualRawAttrs[lastObservedTSAttr]
	require.True(t, ok)
	require.Equal(t, expectedTimestampStr, actualLastObserved)
}

func Test_newResourceAggregator(t *testing.T) {
	attributes := pcommon.NewMap()
	attributes.PutStr("one", "two")
	aggregator := newResourceAggregator(attributes)
	require.NotNil(t, aggregator.logCounters)
	require.Equal(t, attributes, aggregator.attributes)
}

func Test_newLogCounter(t *testing.T) {
	logRecord := plog.NewLogRecord()
	lc := newLogCounter(logRecord)
	require.Equal(t, logRecord, lc.logRecord)
	require.Equal(t, int64(0), lc.count)
}

func Test_getLogKey(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "getLogKey returns the same key for logs that should match",
			testFunc: func(t *testing.T) {
				logRecord1 := generateTestLogRecord(t, "Body of the log")

				//Differ by timestamp
				logRecord1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Minute)))

				logRecord2 := generateTestLogRecord(t, "Body of the log")

				key1 := getLogKey(logRecord1)
				key2 := getLogKey(logRecord2)

				require.Equal(t, key1, key2)
			},
		},
		{
			desc: "getLogKey returns the different key for logs that shouldn't match",
			testFunc: func(t *testing.T) {
				logRecord1 := generateTestLogRecord(t, "Body of the log")

				logRecord2 := generateTestLogRecord(t, "A different Body of the log")

				key1 := getLogKey(logRecord1)
				key2 := getLogKey(logRecord2)

				require.NotEqual(t, key1, key2)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func generateTestLogRecord(t *testing.T, body string) plog.LogRecord {
	t.Helper()
	logRecord := plog.NewLogRecord()
	logRecord.Body().SetStr(body)
	logRecord.SetSeverityText("info")
	logRecord.SetSeverityNumber(0)
	logRecord.Attributes().PutBool("bool", true)
	logRecord.Attributes().PutStr("str", "attr str")
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return logRecord
}
