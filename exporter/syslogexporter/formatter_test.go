// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestCreateFormatter_LineBreaksInMessage(t *testing.T) {
	logRecord := plog.NewLogRecord()
	logRecord.Attributes().PutStr("hostname", "my\nhost")
	logRecord.Attributes().PutStr("message", "first line\nsecond line")
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2003, 8, 24, 5, 14, 15, 0, time.UTC)))

	tests := []struct {
		protocol      string
		network       string
		octetCounting bool
		keepsMessage  bool
	}{
		{protocol: protocolRFC5424Str, network: "tcp"},
		{protocol: protocolRFC5424Str, network: "unix"},
		{protocol: protocolRFC5424Str, network: "tcp", octetCounting: true, keepsMessage: true},
		{protocol: protocolRFC5424Str, network: "udp", keepsMessage: true},
		{protocol: protocolRFC5424Str, network: "unixgram", keepsMessage: true},
		{protocol: protocolRFC3164Str, network: "tcp"},
		{protocol: protocolRFC3164Str, network: "unix"},
		{protocol: protocolRFC3164Str, network: "udp", keepsMessage: true},
		{protocol: protocolRFC3164Str, network: "unixgram", keepsMessage: true},
	}
	for _, tt := range tests {
		formatted := createFormatter(tt.protocol, tt.network, tt.octetCounting).format(logRecord)

		assert.Contains(t, formatted, " my host ", "%+v: header fields never keep line breaks", tt)
		if tt.keepsMessage {
			assert.Contains(t, formatted, "first line\nsecond line\n", "%+v", tt)
		} else {
			assert.Contains(t, formatted, "first line second line\n", "%+v", tt)
		}
	}
}
