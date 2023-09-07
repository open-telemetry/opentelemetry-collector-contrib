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
)

func TestFormatRFC5424(t *testing.T) {

	s := sender{protocol: protocolRFC5424Str}

	msg := map[string]any{
		"timestamp":     "2003-08-24T05:14:15.000003-07:00",
		"appname":       "myproc",
		"facility":      20,
		"hostname":      "192.0.2.1",
		"log.file.name": "syslog",
		"message":       "It's time to make the do-nuts.",
		"priority":      165,
		"proc_id":       "8710",
		"version":       1,
	}

	expected := "<165>1 2003-08-24T05:14:15-07:00 192.0.2.1 myproc 8710 - - It's time to make the do-nuts."
	timeObj1, err := time.Parse(time.RFC3339, "2003-08-24T05:14:15.000003-07:00")
	assert.Equal(t, expected, s.formatRFC5424(msg, timeObj1))
	assert.Nil(t, err)

	msg2 := map[string]any{
		"timestamp":     "2003-10-11T22:14:15.003Z",
		"appname":       "evntslog",
		"facility":      20,
		"hostname":      "mymachine.example.com",
		"log.file.name": "syslog",
		"message":       "BOMAn application event log entry...",
		"msg_id":        "ID47",
		"priority":      165,
		"proc_id":       "111",
		"version":       1,
	}

	expected2 := "<165>1 2003-10-11T22:14:15Z mymachine.example.com evntslog 111 ID47 - BOMAn application event log entry..."
	timeObj2, err := time.Parse(time.RFC3339, "2003-10-11T22:14:15.003Z")
	assert.Nil(t, err)
	assert.Equal(t, expected2, s.formatRFC5424(msg2, timeObj2))

	msg3 := map[string]any{
		"timestamp":     "2003-08-24T05:14:15.000003-07:00",
		"appname":       "myproc",
		"facility":      20,
		"hostname":      "192.0.2.1",
		"log.file.name": "syslog",
		"message":       "It's time to make the do-nuts.",
		"priority":      165,
		"proc_id":       "8710",
		"version":       1,
		"structured_data": map[string]map[string]string{
			"SecureAuth@27389": {
				"PEN":             "27389",
				"Realm":           "SecureAuth0",
				"UserHostAddress": "192.168.2.132",
				"UserID":          "Tester2",
			},
		},
	}

	expectedForm := "\\<165\\>1 2003-08-24T05:14:15-07:00 192\\.0\\.2\\.1 myproc 8710 - " +
		"\\[\\S+ \\S+ \\S+ \\S+ \\S+\\] It's time to make the do-nuts\\."
	timeObj3, err := time.Parse(time.RFC3339, "2003-08-24T05:14:15.000003-07:00")
	assert.Nil(t, err)
	formattedMsg := s.formatRFC5424(msg3, timeObj3)
	matched, err := regexp.MatchString(expectedForm, formattedMsg)
	assert.Nil(t, err)
	assert.Equal(t, true, matched, fmt.Sprintf("unexpected form of formatted message, formatted message: %s, regexp: %s", formattedMsg, expectedForm))
	assert.Equal(t, true, strings.Contains(formattedMsg, "Realm=\"SecureAuth0\""))
	assert.Equal(t, true, strings.Contains(formattedMsg, "UserHostAddress=\"192.168.2.132\""))
	assert.Equal(t, true, strings.Contains(formattedMsg, "UserID=\"Tester2\""))
	assert.Equal(t, true, strings.Contains(formattedMsg, "PEN=\"27389\""))
}
