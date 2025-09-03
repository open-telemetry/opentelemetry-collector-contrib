// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestMarshalerMissingAttributes(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty()
	marshaler := &sumoMarshaler{}
	require.NotNil(t, marshaler)
	_, err := marshaler.MarshalLogs(logs)
	assert.Error(t, err)
}

func TestMarshalerMissingSourceHost(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	rls.Resource().Attributes().PutStr("_sourceCategory", "testcategory")

	marshaler := &sumoMarshaler{}
	require.NotNil(t, marshaler)
	_, err := marshaler.MarshalLogs(logs)
	assert.Error(t, err)
}

func TestMarshalerMissingScopedLogs(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	rls.Resource().Attributes().PutStr("_sourceCategory", "testcategory")
	rls.Resource().Attributes().PutStr("_sourceHost", "testHost")
	rls.Resource().Attributes().PutStr("_sourceName", "testName")

	marshaler := &sumoMarshaler{}
	require.NotNil(t, marshaler)
	_, err := marshaler.MarshalLogs(logs)
	assert.NoError(t, err)
}

func TestMarshalerMissingSourceName(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	rls.Resource().Attributes().PutStr("_sourceCategory", "testcategory")
	rls.Resource().Attributes().PutStr("_sourceHost", "testHost")

	sl := rls.ScopeLogs().AppendEmpty()
	const recordNum = 0

	ts := pcommon.Timestamp(int64(recordNum) * time.Millisecond.Nanoseconds())
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("entry1")
	logRecord.SetTimestamp(ts)

	marshaler := &sumoMarshaler{}
	require.NotNil(t, marshaler)
	_, err := marshaler.MarshalLogs(logs)
	assert.Error(t, err)
}

func TestMarshalerOkStructure(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	rls.Resource().Attributes().PutStr("_sourceCategory", "testcategory")
	rls.Resource().Attributes().PutStr("_sourceHost", "testHost")
	rls.Resource().Attributes().PutStr("_sourceName", "testSourceName")
	rls.Resource().Attributes().PutStr("42", "the question")
	slice := rls.Resource().Attributes().PutEmptySlice("slice")
	pcommon.NewValueInt(13).CopyTo(slice.AppendEmpty())
	m := pcommon.NewValueMap()
	m.Map().PutBool("b", true)
	m.CopyTo(slice.AppendEmpty())

	sl := rls.ScopeLogs().AppendEmpty()
	const recordNum = 0

	ts := pcommon.Timestamp(int64(recordNum) * time.Millisecond.Nanoseconds())
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("entry1")
	logRecord.SetTimestamp(ts)
	logRecord.Attributes().PutStr("key", "value")

	marshaler := &sumoMarshaler{}
	require.NotNil(t, marshaler)
	buf, err := marshaler.MarshalLogs(logs)
	assert.NoError(t, err)
	expectedEntry := "{\"date\": \"1970-01-01 00:00:00 +0000 UTC\",\"sourceName\":\"testSourceName\",\"sourceHost\":\"testHost\""
	expectedEntry += ",\"sourceCategory\":\"testcategory\",\"fields\":{\"42\":\"the question\",\"slice\":[13,{\"b\":true}]},\"message\":{\"key\":\"value\",\"log\":\"entry1\"}}\n"
	assert.Equal(t, expectedEntry, string(buf))
}

func TestMarshalerQuotes(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	rls.Resource().Attributes().PutStr("_sourceCategory", `"foo"bar"`)
	rls.Resource().Attributes().PutStr("_sourceHost", "testHost")
	rls.Resource().Attributes().PutStr("_sourceName", "testSourceName")

	sl := rls.ScopeLogs().AppendEmpty()
	const recordNum = 0

	ts := pcommon.Timestamp(int64(recordNum) * time.Millisecond.Nanoseconds())
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("entry1")
	logRecord.SetTimestamp(ts)

	marshaler := &sumoMarshaler{}
	require.NotNil(t, marshaler)
	buf, err := marshaler.MarshalLogs(logs)
	assert.NoError(t, err)
	expectedEntry := "{\"date\": \"1970-01-01 00:00:00 +0000 UTC\",\"sourceName\":\"testSourceName\",\"sourceHost\":\"testHost\""
	expectedEntry += ",\"sourceCategory\":\"\\\"foo\\\"bar\\\"\",\"fields\":{},\"message\":{\"log\":\"entry1\"}}\n"
	assert.Equal(t, expectedEntry, string(buf))
}

func TestAttributeValueToString(t *testing.T) {
	testCases := []struct {
		value  pcommon.Value
		result string
		init   func(pcommon.Value)
	}{
		{
			value:  pcommon.NewValueBool(true),
			result: "true",
		},
		{
			value:  pcommon.NewValueBytes(),
			result: "\"KiFN/wA=\"",
			init: func(v pcommon.Value) {
				v.Bytes().Append(42, 33, 77, 255, 0)
			},
		},
		{
			value:  pcommon.NewValueDouble(1.69),
			result: "1.69",
		},
		{
			value:  pcommon.NewValueInt(42),
			result: "42",
		},
		{
			// Format of a map entry:
			// "     -> <key>: <type>(<value>)\n"
			// Type names: https://github.com/open-telemetry/opentelemetry-collector/blob/ed8547a8e5d6ed527e6d54136cb2e137b954f888/pdata/pcommon/value.go#L32
			value: pcommon.NewValueMap(),
			result: "{" +
				"\"bool\":false," +
				"\"map\":{}," +
				"\"string\":\"abc\"" +
				"}",
			init: func(v pcommon.Value) {
				m := v.Map()
				m.PutBool("bool", false)
				m.PutEmptyMap("map")
				m.PutStr("string", "abc")
			},
		},
		{
			value:  pcommon.NewValueSlice(),
			result: "[110.37,[true],\"YWJj\",\"asdfg\"]",
			init: func(v pcommon.Value) {
				s := v.Slice()
				s.AppendEmpty().SetDouble(110.37)
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetBool(true)
				s.AppendEmpty().SetEmptyBytes().Append(97, 98, 99)
				s.AppendEmpty().SetStr("asdfg")
			},
		},
		{
			value:  pcommon.NewValueStr("qwerty"),
			result: "qwerty",
		},
	}

	for _, testCase := range testCases {
		if testCase.init != nil {
			testCase.init(testCase.value)
		}
		val, err := attributeValueToString(testCase.value)
		assert.NoError(t, err)
		assert.Equal(t, testCase.result, val)
	}
}

func TestMarshalerNonMutatingFlow(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()

	ra := rls.Resource().Attributes()
	ra.PutStr("_sourceCategory", "test/category")
	ra.PutStr("_sourceHost", "test-host")
	ra.PutStr("_sourceName", "test-source")
	ra.PutStr("service.name", "test-service")
	ra.PutStr("service.version", "1.0.0")
	ra.PutInt("custom.number", 42)

	sl := rls.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test log message")
	logRecord.SetTimestamp(pcommon.Timestamp(time.Now().UnixNano()))
	logRecord.Attributes().PutStr("log.level", "info")
	logRecord.Attributes().PutStr("user.id", "test-user")

	originalResourceAttrs := make(map[string]any)
	ra.Range(func(k string, v pcommon.Value) bool {
		originalResourceAttrs[k] = v.AsRaw()
		return true
	})

	originalLogAttrs := make(map[string]any)
	logRecord.Attributes().Range(func(k string, v pcommon.Value) bool {
		originalLogAttrs[k] = v.AsRaw()
		return true
	})

	originalLogBody := logRecord.Body().AsRaw()

	marshaler := &sumoMarshaler{}
	require.NotNil(t, marshaler)
	buf, err := marshaler.MarshalLogs(logs)
	require.NoError(t, err)
	require.NotEmpty(t, buf)

	t.Run("ResourceAttributesUnchanged", func(t *testing.T) {
		currentAttrs := make(map[string]any)
		ra.Range(func(k string, v pcommon.Value) bool {
			currentAttrs[k] = v.AsRaw()
			return true
		})

		assert.Equal(t, originalResourceAttrs, currentAttrs, "Resource attributes were modified during marshaling")

		sourceCategory, exists := ra.Get("_sourceCategory")
		assert.True(t, exists, "_sourceCategory should still exist")
		assert.Equal(t, "test/category", sourceCategory.Str())

		sourceHost, exists := ra.Get("_sourceHost")
		assert.True(t, exists, "_sourceHost should still exist")
		assert.Equal(t, "test-host", sourceHost.Str())

		sourceName, exists := ra.Get("_sourceName")
		assert.True(t, exists, "_sourceName should still exist")
		assert.Equal(t, "test-source", sourceName.Str())
	})

	t.Run("LogRecordAttributesUnchanged", func(t *testing.T) {
		currentLogAttrs := make(map[string]any)
		logRecord.Attributes().Range(func(k string, v pcommon.Value) bool {
			currentLogAttrs[k] = v.AsRaw()
			return true
		})

		assert.Equal(t, originalLogAttrs, currentLogAttrs, "Log record attributes were modified during marshaling")

		_, logKeyExists := logRecord.Attributes().Get("log")
		assert.False(t, logKeyExists, "Original log record should not have 'log' key added")
	})

	t.Run("LogBodyUnchanged", func(t *testing.T) {
		currentLogBody := logRecord.Body().AsRaw()
		assert.Equal(t, originalLogBody, currentLogBody, "Log body was modified during marshaling")
	})

	t.Run("OutputFormatCorrect", func(t *testing.T) {
		output := string(buf)

		assert.Contains(t, output, `"sourceCategory":"test/category"`)
		assert.Contains(t, output, `"sourceHost":"test-host"`)
		assert.Contains(t, output, `"sourceName":"test-source"`)

		assert.Contains(t, output, `"service.name":"test-service"`)
		assert.Contains(t, output, `"service.version":"1.0.0"`)
		assert.Contains(t, output, `"custom.number":42`)

		assert.NotContains(t, output, `"fields":{"_sourceCategory"`)
		assert.NotContains(t, output, `"fields":{"_sourceHost"`)
		assert.NotContains(t, output, `"fields":{"_sourceName"`)

		assert.Contains(t, output, `"log.level":"info"`)
		assert.Contains(t, output, `"user.id":"test-user"`)
		assert.Contains(t, output, `"log":"test log message"`)
	})
}

func TestMarshalerMultipleCallsSafe(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()

	ra := rls.Resource().Attributes()
	ra.PutStr("_sourceCategory", "test/category")
	ra.PutStr("_sourceHost", "test-host")
	ra.PutStr("_sourceName", "test-source")
	ra.PutStr("service.name", "test-service")

	sl := rls.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test message")
	logRecord.Attributes().PutStr("key", "value")

	marshaler := &sumoMarshaler{}

	buf1, err1 := marshaler.MarshalLogs(logs)
	require.NoError(t, err1)

	buf2, err2 := marshaler.MarshalLogs(logs)
	require.NoError(t, err2)

	buf3, err3 := marshaler.MarshalLogs(logs)
	require.NoError(t, err3)

	assert.Equal(t, string(buf1), string(buf2), "Multiple marshaling calls should produce identical output")
	assert.Equal(t, string(buf2), string(buf3), "Multiple marshaling calls should produce identical output")

	sourceCategory, exists := ra.Get("_sourceCategory")
	assert.True(t, exists)
	assert.Equal(t, "test/category", sourceCategory.Str())
}

func TestMarshalerEmptyAttributes(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()

	ra := rls.Resource().Attributes()
	ra.PutStr("_sourceCategory", "minimal")
	ra.PutStr("_sourceHost", "host")
	ra.PutStr("_sourceName", "name")

	sl := rls.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("minimal log")

	marshaler := &sumoMarshaler{}
	buf, err := marshaler.MarshalLogs(logs)
	require.NoError(t, err)

	output := string(buf)

	assert.Contains(t, output, `"fields":{}`)

	assert.Contains(t, output, `"message":{"log":"minimal log"}`)

	assert.Equal(t, 3, ra.Len(), "Should still have exactly 3 source attributes")
}
