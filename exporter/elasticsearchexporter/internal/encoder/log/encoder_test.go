// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var (
	expectedLogBody                           = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`
	expectedLogBodyWithEmptyTimestamp         = `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`
	expectedLogBodyDeDottedWithEmptyTimestamp = `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Attributes":{"log-attr1":"value1"},"Body":"log-body","Resource":{"foo":{"bar":"baz"},"key1":"value1"},"Scope":{"name":"","version":""},"SeverityNumber":0,"TraceFlags":0}`
)

func TestEncodeLog(t *testing.T) {
	t.Run("empty timestamp with observedTimestamp override", func(t *testing.T) {
		e := &defaultEncoder{}
		td := mockResourceLogs()
		td.ScopeLogs().At(0).LogRecords().At(0).SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)))
		logByte, err := e.EncodeLog(td.Resource(), td.SchemaUrl(), td.ScopeLogs().At(0).LogRecords().At(0), td.ScopeLogs().At(0).Scope(), td.ScopeLogs().At(0).SchemaUrl())
		assert.NoError(t, err)
		assert.Equal(t, expectedLogBody, string(logByte))
	})

	t.Run("both timestamp and observedTimestamp empty", func(t *testing.T) {
		e := &defaultEncoder{}
		td := mockResourceLogs()
		logByte, err := e.EncodeLog(td.Resource(), td.SchemaUrl(), td.ScopeLogs().At(0).LogRecords().At(0), td.ScopeLogs().At(0).Scope(), td.ScopeLogs().At(0).SchemaUrl())
		assert.NoError(t, err)
		assert.Equal(t, expectedLogBodyWithEmptyTimestamp, string(logByte))
	})

	t.Run("dedot true", func(t *testing.T) {
		e := &defaultEncoder{}
		td := mockResourceLogs()
		td.Resource().Attributes().PutStr("foo.bar", "baz")
		logByte, err := e.EncodeLog(td.Resource(), td.SchemaUrl(), td.ScopeLogs().At(0).LogRecords().At(0), td.ScopeLogs().At(0).Scope(), td.ScopeLogs().At(0).SchemaUrl())
		require.NoError(t, err)
		require.Equal(t, expectedLogBodyDeDottedWithEmptyTimestamp, string(logByte))
	})
}

func mockResourceLogs() plog.ResourceLogs {
	rl := plog.NewResourceLogs()
	rl.Resource().Attributes().PutStr("key1", "value1")
	l := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	l.Attributes().PutStr("log-attr1", "value1")
	l.Body().SetStr("log-body")
	return rl
}

func TestEncodeLogScalarObjectConflict(t *testing.T) {
	// If there is an attribute named "foo", and another called "foo.bar",
	// then "foo" will be renamed to "foo.value".
	e := &defaultEncoder{}
	td := mockResourceLogs()
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo", "scalar")
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo.bar", "baz")
	encoded, err := e.EncodeLog(td.Resource(), "", td.ScopeLogs().At(0).LogRecords().At(0), td.ScopeLogs().At(0).Scope(), "")
	assert.NoError(t, err)

	assert.True(t, gjson.ValidBytes(encoded))
	assert.False(t, gjson.GetBytes(encoded, "Attributes\\.foo").Exists())
	fooValue := gjson.GetBytes(encoded, "Attributes\\.foo\\.value")
	fooBar := gjson.GetBytes(encoded, "Attributes\\.foo\\.bar")
	assert.Equal(t, "scalar", fooValue.Str)
	assert.Equal(t, "baz", fooBar.Str)

	// If there is an attribute named "foo.value", then "foo" would be omitted rather than renamed.
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo.value", "foovalue")
	encoded, err = e.EncodeLog(td.Resource(), "", td.ScopeLogs().At(0).LogRecords().At(0), td.ScopeLogs().At(0).Scope(), "")
	assert.NoError(t, err)

	assert.False(t, gjson.GetBytes(encoded, "Attributes\\.foo").Exists())
	fooValue = gjson.GetBytes(encoded, "Attributes\\.foo\\.value")
	assert.Equal(t, "foovalue", fooValue.Str)
}
