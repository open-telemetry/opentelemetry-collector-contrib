// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor/internal/metadata"
)

func Test_newLogAggregator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	aggregator := newLogAggregator(cfg.LogCountAttribute, time.UTC, telemetryBuilder, cfg.IncludeFields)
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

	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	// Setup aggregator
	aggregator := newLogAggregator("log_count", time.UTC, telemetryBuilder, nil)
	logRecord := plog.NewLogRecord()

	resource := pcommon.NewResource()
	resource.Attributes().PutStr("one", "two")

	scope := pcommon.NewInstrumentationScope()

	expectedResourceKey := getResourceKey(resource)
	expectedScopeKey := getScopeKey(scope)
	expectedLogKey := getLogKey(logRecord, nil)

	// Add logRecord
	aggregator.Add(resource, scope, logRecord)

	// Check resourceCounter was set
	resourceCounter, ok := aggregator.resources[expectedResourceKey]
	require.True(t, ok)
	require.Equal(t, resource, resourceCounter.resource)

	// check scopeCounter was set
	scopeCounter, ok := resourceCounter.scopeCounters[expectedScopeKey]
	require.True(t, ok)
	require.Equal(t, scope, scopeCounter.scope)

	// Check logCounter was set
	lc, ok := scopeCounter.logCounters[expectedLogKey]
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

	aggregator.Add(resource, scope, logRecord)
	require.Equal(t, int64(2), lc.count)
	require.Equal(t, secondExpectedTimestamp, lc.lastObservedTimestamp)
}

func Test_logAggregatorReset(t *testing.T) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	aggregator := newLogAggregator("log_count", time.UTC, telemetryBuilder, nil)
	for i := 0; i < 2; i++ {
		resource := pcommon.NewResource()
		resource.Attributes().PutInt("i", int64(i))
		key := getResourceKey(resource)
		aggregator.resources[key] = newResourceAggregator(resource, nil)
	}

	require.Len(t, aggregator.resources, 2)

	aggregator.Reset()

	require.Empty(t, aggregator.resources)
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
	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	aggregator := newLogAggregator(defaultLogCountAttribute, location, telemetryBuilder, nil)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("one", "two")
	expectedHash := pdatautil.MapHash(resource.Attributes())

	scope := pcommon.NewInstrumentationScope()

	logRecord := generateTestLogRecord(t, "body string")

	// Add logRecord
	aggregator.Add(resource, scope, logRecord)

	exportedLogs := aggregator.Export(context.Background())
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
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("one", "two")
	aggregator := newResourceAggregator(resource, nil)
	require.NotNil(t, aggregator.scopeCounters)
	require.Equal(t, resource, aggregator.resource)
}

func Test_newScopeCounter(t *testing.T) {
	scope := pcommon.NewInstrumentationScope()
	scope.Attributes().PutStr("one", "two")
	sc := newScopeAggregator(scope, nil)
	require.Equal(t, scope, sc.scope)
	require.NotNil(t, sc.logCounters)
}

func Test_newLogCounter(t *testing.T) {
	oldTimeNow := timeNow
	defer func() {
		timeNow = oldTimeNow
	}()

	now := time.Now().UTC()
	timeNow = func() time.Time { return now }
	logRecord := plog.NewLogRecord()
	lc := newLogCounter(logRecord)
	require.Equal(t, logRecord, lc.logRecord)
	require.Equal(t, int64(0), lc.count)
	require.Equal(t, now, lc.firstObservedTimestamp)
	require.Equal(t, now, lc.lastObservedTimestamp)
}

func Test_logCounterIncrement(t *testing.T) {
	oldTimeNow := timeNow
	defer func() {
		timeNow = oldTimeNow
	}()

	first := time.Now().UTC()
	timeNow = func() time.Time { return first }
	logRecord := plog.NewLogRecord()
	lc := newLogCounter(logRecord)
	require.Equal(t, logRecord, lc.logRecord)
	require.Equal(t, int64(0), lc.count)
	require.Equal(t, first, lc.firstObservedTimestamp)

	last := time.Now().UTC()
	timeNow = func() time.Time { return last }
	lc.Increment()
	require.Equal(t, int64(1), lc.count)
	require.Equal(t, first, lc.firstObservedTimestamp)
	require.Equal(t, last, lc.lastObservedTimestamp)
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

				// Differ by timestamp
				logRecord1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Minute)))

				logRecord2 := generateTestLogRecord(t, "Body of the log")

				key1 := getLogKey(logRecord1, nil)
				key2 := getLogKey(logRecord2, nil)

				require.Equal(t, key1, key2)
			},
		},
		{
			desc: "getLogKey returns the different key for logs that shouldn't match",
			testFunc: func(t *testing.T) {
				logRecord1 := generateTestLogRecord(t, "Body of the log")

				logRecord2 := generateTestLogRecord(t, "A different Body of the log")

				key1 := getLogKey(logRecord1, nil)
				key2 := getLogKey(logRecord2, nil)

				require.NotEqual(t, key1, key2)
			},
		},
		{
			desc: "getLogKey returns the dedup key",
			testFunc: func(t *testing.T) {
				dedupValue := "abc123"
				logRecord := generateTestLogRecordWithMap(t)
				logRecord.Body().Map().PutStr("dedup_key", dedupValue)
				logRecord.Attributes().PutStr("dedup_key", dedupValue)

				expected := pdatautil.Hash64(
					pdatautil.WithString(dedupValue),
				)
				expectedMulti := pdatautil.Hash64(
					pdatautil.WithString(dedupValue),
					pdatautil.WithString(dedupValue),
				)

				require.Equal(t, expected, getLogKey(logRecord, []string{"body.dedup_key"}))
				require.Equal(t, expected, getLogKey(logRecord, []string{"attributes.dedup_key"}))
				require.Equal(t, expectedMulti, getLogKey(logRecord, []string{"body.dedup_key", "attributes.dedup_key"}))
			},
		},
		{
			desc: "getLogKey hashes full message if dedup key is body-based and no body was provided",
			testFunc: func(t *testing.T) {
				logRecord := plog.NewLogRecord()
				logRecord.Attributes().PutStr("str", "attr str")

				expected := pdatautil.Hash64(
					pdatautil.WithMap(logRecord.Attributes()),
					pdatautil.WithValue(logRecord.Body()),
					pdatautil.WithString(logRecord.SeverityNumber().String()),
					pdatautil.WithString(logRecord.SeverityText()),
				)

				require.Equal(t, expected, getLogKey(logRecord, []string{"body.dedup_key"}))
			},
		},
		{
			desc: "getLogKey hashes full message if dedup key is body-based and body is not map type",
			testFunc: func(t *testing.T) {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("hello, this is a message body string")

				expected := pdatautil.Hash64(
					pdatautil.WithMap(logRecord.Attributes()),
					pdatautil.WithValue(logRecord.Body()),
					pdatautil.WithString(logRecord.SeverityNumber().String()),
					pdatautil.WithString(logRecord.SeverityText()),
				)

				require.Equal(t, expected, getLogKey(logRecord, []string{"body.dedup_key"}))
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

func generateTestLogRecordWithMap(t *testing.T) plog.LogRecord {
	t.Helper()
	logRecord := generateTestLogRecord(t, "")
	logRecord.Body().SetEmptyMap()
	return logRecord
}
