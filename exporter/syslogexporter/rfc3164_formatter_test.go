// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestRFC3164Formatter(t *testing.T) {
	expected := "<34>Aug 24 05:14:15 mymachine su: 'su root' failed for lonvick on /dev/pts/8\n"
	logRecord := plog.NewLogRecord()
	logRecord.Attributes().PutStr("appname", "su")
	logRecord.Attributes().PutStr("hostname", "mymachine")
	logRecord.Attributes().PutStr("message", "'su root' failed for lonvick on /dev/pts/8")
	logRecord.Attributes().PutInt("priority", 34)
	timestamp, err := time.Parse(time.RFC3339Nano, "2003-08-24T05:14:15.000003Z")
	require.NoError(t, err)
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

	actual := newRFC3164Formatter().format(logRecord)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	expected = "<165>Aug 24 05:14:15 - -\n"
	logRecord = plog.NewLogRecord()
	logRecord.Attributes().PutStr("message", "-")
	timestamp, err = time.Parse(time.RFC3339Nano, "2003-08-24T05:14:15.000003Z")
	require.NoError(t, err)
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

	actual = newRFC3164Formatter().format(logRecord)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}
