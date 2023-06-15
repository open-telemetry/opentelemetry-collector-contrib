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

	sl := rls.ScopeLogs().AppendEmpty()
	const recordNum = 0

	ts := pcommon.Timestamp(int64(recordNum) * time.Millisecond.Nanoseconds())
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("entry1")
	logRecord.Attributes().PutStr("log.file.path_resolved", "testSourceName")
	logRecord.SetTimestamp(ts)

	marshaler := &SumoMarshaler{"txt"}
	require.NotNil(t, marshaler)
	buf, err := marshaler.MarshalLogs(logs)
	assert.NoError(t, err)
	expectedEntry := "{\"data\": \"1970-01-01 00:00:00 +0000 UTC\",\"sourceName\":\"testSourceName\",\"sourceHost\":\"testHost\""
	expectedEntry = expectedEntry + ",\"sourceCategory\":\"testcategory\",\"fields\":{},\"message\":\"entry1\"}\n"
	assert.Equal(t, string(buf), expectedEntry)
}
