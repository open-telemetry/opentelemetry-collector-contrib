// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestRFC5424Formatter(t *testing.T) {
	expected := "<165>1 2003-08-24T05:14:15.000003Z 192.0.2.1 myproc 8710 - - It's time to make the do-nuts.\n"
	logRecord := plog.NewLogRecord()
	logRecord.Attributes().PutStr("appname", "myproc")
	logRecord.Attributes().PutStr("hostname", "192.0.2.1")
	logRecord.Attributes().PutStr("message", "It's time to make the do-nuts.")
	logRecord.Attributes().PutInt("priority", 165)
	logRecord.Attributes().PutStr("proc_id", "8710")
	logRecord.Attributes().PutInt("version", 1)
	timestamp, err := time.Parse(time.RFC3339Nano, "2003-08-24T05:14:15.000003Z")
	require.NoError(t, err)
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

	actual := newRFC5424Formatter().format(logRecord)
	assert.Equal(t, expected, actual)

	expected = "<165>1 2003-10-11T22:14:15.003Z mymachine.example.com evntslog 111 ID47 - BOMAn application event log entry...\n"
	logRecord = plog.NewLogRecord()
	logRecord.Attributes().PutStr("appname", "evntslog")
	logRecord.Attributes().PutStr("hostname", "mymachine.example.com")
	logRecord.Attributes().PutStr("message", "BOMAn application event log entry...")
	logRecord.Attributes().PutStr("msg_id", "ID47")
	logRecord.Attributes().PutInt("priority", 165)
	logRecord.Attributes().PutStr("proc_id", "111")
	logRecord.Attributes().PutInt("version", 1)
	timestamp, err = time.Parse(time.RFC3339Nano, "2003-10-11T22:14:15.003Z")
	require.NoError(t, err)
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

	actual = newRFC5424Formatter().format(logRecord)
	assert.Equal(t, expected, actual)

	// Test structured data
	expectedRegex := "\\<165\\>1 2003-08-24T12:14:15.000003Z 192\\.0\\.2\\.1 myproc 8710 - " +
		"\\[\\S+ \\S+ \\S+ \\S+ \\S+\\] It's time to make the do-nuts\\.\n"
	logRecord = plog.NewLogRecord()
	logRecord.Attributes().PutStr("appname", "myproc")
	logRecord.Attributes().PutStr("hostname", "192.0.2.1")
	logRecord.Attributes().PutStr("message", "It's time to make the do-nuts.")
	logRecord.Attributes().PutInt("priority", 165)
	logRecord.Attributes().PutStr("proc_id", "8710")
	logRecord.Attributes().PutEmptyMap("structured_data")
	structuredData, found := logRecord.Attributes().Get("structured_data")
	require.True(t, found)
	structuredData.Map().PutEmptyMap("SecureAuth@27389")
	structuredDataSubmap, found := structuredData.Map().Get("SecureAuth@27389")
	require.True(t, found)
	structuredDataSubmap.Map().PutStr("PEN", "27389")
	structuredDataSubmap.Map().PutStr("Realm", "SecureAuth0")
	structuredDataSubmap.Map().PutStr("UserHostAddress", "192.168.2.132")
	structuredDataSubmap.Map().PutStr("UserID", "Tester2")
	logRecord.Attributes().PutInt("version", 1)
	timestamp, err = time.Parse(time.RFC3339Nano, "2003-08-24T05:14:15.000003-07:00")
	require.NoError(t, err)
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

	actual = newRFC5424Formatter().format(logRecord)
	assert.NoError(t, err)
	matched, err := regexp.MatchString(expectedRegex, actual)
	assert.NoError(t, err)
	assert.True(t, matched, fmt.Sprintf("unexpected form of formatted message, formatted message: %s, regexp: %s", actual, expectedRegex))
	assert.True(t, strings.Contains(actual, "Realm=\"SecureAuth0\""))
	assert.True(t, strings.Contains(actual, "UserHostAddress=\"192.168.2.132\""))
	assert.True(t, strings.Contains(actual, "UserID=\"Tester2\""))
	assert.True(t, strings.Contains(actual, "PEN=\"27389\""))

	// Test defaults
	expected = "<165>1 2003-08-24T12:14:15.000003Z - - - - -\n"
	logRecord = plog.NewLogRecord()
	timestamp, err = time.Parse(time.RFC3339Nano, "2003-08-24T05:14:15.000003-07:00")
	require.NoError(t, err)
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

	actual = newRFC5424Formatter().format(logRecord)
	assert.Equal(t, expected, actual)
}
