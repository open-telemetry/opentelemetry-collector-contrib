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
	marshaler := &SumoMarshaler{"txt"}
	require.NotNil(t, marshaler)
	_, err := marshaler.MarshalLogs(logs)
	assert.Error(t, err)
}

func TestMarshalerMissingSourceHost(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	rls.Resource().Attributes().PutStr("_sourceCategory", "testcategory")

	marshaler := &SumoMarshaler{"txt"}
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

	marshaler := &SumoMarshaler{"txt"}
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

	marshaler := &SumoMarshaler{"txt"}
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

	sl := rls.ScopeLogs().AppendEmpty()
	const recordNum = 0

	ts := pcommon.Timestamp(int64(recordNum) * time.Millisecond.Nanoseconds())
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("entry1")
	logRecord.SetTimestamp(ts)

	marshaler := &SumoMarshaler{"txt"}
	require.NotNil(t, marshaler)
	buf, err := marshaler.MarshalLogs(logs)
	assert.NoError(t, err)
	expectedEntry := "{\"date\": \"1970-01-01 00:00:00 +0000 UTC\",\"sourceName\":\"testSourceName\",\"sourceHost\":\"testHost\""
	expectedEntry += ",\"sourceCategory\":\"testcategory\",\"fields\":{},\"message\":\"entry1\"}\n"
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
			result: "[42, 33, 77, 255, 0]",
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
			result: "{\n" +
				"     -> bool: Bool(false)\n" +
				"     -> map: Map({})\n" +
				"     -> string: Str(abc)\n" +
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
			result: "[110.37, [true], [1, 2, 3], asdfg]",
			init: func(v pcommon.Value) {
				s := v.Slice()
				s.AppendEmpty().SetDouble(110.37)
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetBool(true)
				s.AppendEmpty().SetEmptyBytes().Append(1, 2, 3)
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
		assert.Equal(t, attributeValueToString(testCase.value), testCase.result)
	}
}
