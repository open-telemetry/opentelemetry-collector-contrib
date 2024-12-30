// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestEncodeLogBodyMapMode(t *testing.T) {
	resource := pcommon.NewResource()
	scope := pcommon.NewInstrumentationScope()

	// craft a log record with a body map
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecords := scopeLogs.LogRecords()
	observedTimestamp := pcommon.Timestamp(time.Now().UnixNano()) // nolint:gosec // UnixNano is positive and thus safe to convert to signed integer.

	logRecord := logRecords.AppendEmpty()
	logRecord.SetObservedTimestamp(observedTimestamp)

	bodyMap := pcommon.NewMap()
	bodyMap.PutStr("@timestamp", "2024-03-12T20:00:41.123456789Z")
	bodyMap.PutInt("id", 1)
	bodyMap.PutStr("key", "value")
	bodyMap.PutStr("key.a", "a")
	bodyMap.PutStr("key.a.b", "b")
	bodyMap.PutDouble("pi", 3.14)
	bodyMap.CopyTo(logRecord.Body().SetEmptyMap())

	e := bodyMapEncoder{}
	got, err := e.EncodeLog(resource, "", logRecord, scope, "")
	require.NoError(t, err)

	require.JSONEq(t, `{
		"@timestamp":                 "2024-03-12T20:00:41.123456789Z",
		"id":                         1,
		"key":                        "value",
		"key.a":                      "a",
		"key.a.b":                    "b",
		"pi":                         3.14
	}`, string(got))

	// invalid body map
	logRecord.Body().SetEmptySlice()
	_, err = e.EncodeLog(resource, "", logRecord, scope, "")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidTypeForBodyMapMode)
}
