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

func TestBodyMarshalerWithBooleanType(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	sl := rls.ScopeLogs().AppendEmpty()

	const recordNum = 0
	ts := pcommon.Timestamp(int64(recordNum) * time.Millisecond.Nanoseconds())
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(ts)

	// Boolean
	logRecord.Body().SetBool(true)

	marshaler := &bodyMarshaler{}
	require.NotNil(t, marshaler)
	body, err := marshaler.MarshalLogs(logs)
	assert.NoError(t, err)
	assert.Equal(t, body, []byte("true\n"))
}

func TestBodyMarshalerWithNumberType(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	sl := rls.ScopeLogs().AppendEmpty()

	const recordNum = 0
	ts := pcommon.Timestamp(int64(recordNum) * time.Millisecond.Nanoseconds())
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(ts)

	// Number
	logRecord.Body().SetDouble(0.05)

	marshaler := &bodyMarshaler{}
	require.NotNil(t, marshaler)
	body, err := marshaler.MarshalLogs(logs)
	assert.NoError(t, err)
	assert.Equal(t, body, []byte("0.05\n"))
}

func TestBodyMarshalerWithMapType(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	sl := rls.ScopeLogs().AppendEmpty()

	const recordNum = 0
	ts := pcommon.Timestamp(int64(recordNum) * time.Millisecond.Nanoseconds())
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(ts)

	// Map
	m := logRecord.Body().SetEmptyMap()
	m.PutStr("foo", "foo")
	m.PutStr("bar", "bar")
	m.PutBool("foobar", false)
	m.PutDouble("foobardouble", 0.006)
	m.PutInt("foobarint", 1)

	expect := `{"bar":"bar","foo":"foo","foobar":false,"foobardouble":0.006,"foobarint":1}`

	marshaler := &bodyMarshaler{}
	require.NotNil(t, marshaler)
	body, err := marshaler.MarshalLogs(logs)
	assert.NoError(t, err)
	assert.Equal(t, body, []byte(expect+"\n"))
}
