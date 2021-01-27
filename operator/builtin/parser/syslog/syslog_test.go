// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syslog

import (
	"context"
	"testing"
	"time"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

func TestSyslogParser(t *testing.T) {
	basicConfig := func() *SyslogParserConfig {
		cfg := NewSyslogParserConfig("test_operator_id")
		cfg.OutputIDs = []string{"fake"}
		return cfg
	}

	cases := []struct {
		name                 string
		config               *SyslogParserConfig
		inputRecord          interface{}
		expectedTimestamp    time.Time
		expectedRecord       interface{}
		expectedSeverity     entry.Severity
		expectedSeverityText string
	}{
		{
			"RFC3164",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc3164"
				return cfg
			}(),
			"<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, time.UTC),
			map[string]interface{}{
				"appname":  "apache_server",
				"facility": 4,
				"hostname": "1.2.3.4",
				"message":  "test message",
				"priority": 34,
			},
			entry.Critical,
			"crit",
		},
		{
			"RFC3164Bytes",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc3164"
				return cfg
			}(),
			[]byte("<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message"),
			time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, time.UTC),
			map[string]interface{}{
				"appname":  "apache_server",
				"facility": 4,
				"hostname": "1.2.3.4",
				"message":  "test message",
				"priority": 34,
			},
			entry.Critical,
			"crit",
		},
		{
			"RFC5424",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc5424"
				return cfg
			}(),
			`<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
			map[string]interface{}{
				"appname":  "SecureAuth0",
				"facility": 10,
				"hostname": "192.168.2.132",
				"message":  "Found the user for retrieving user's profile",
				"msg_id":   "ID52020",
				"priority": 86,
				"proc_id":  "23108",
				"structured_data": map[string]map[string]string{
					"SecureAuth@27389": {
						"PEN":             "27389",
						"Realm":           "SecureAuth0",
						"UserHostAddress": "192.168.2.132",
						"UserID":          "Tester2",
					},
				},
				"version": 1,
			},
			entry.Info,
			"info",
		},
		{
			"RFC5424LongSDName",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc5424"
				return cfg
			}(),
			`<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [verylongsdnamethatisgreaterthan32bytes@12345 UserHostAddress="192.168.2.132"] my message`,
			time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
			map[string]interface{}{
				"appname":  "SecureAuth0",
				"facility": 10,
				"hostname": "192.168.2.132",
				"message":  "my message",
				"msg_id":   "ID52020",
				"priority": 86,
				"proc_id":  "23108",
				"structured_data": map[string]map[string]string{
					"verylongsdnamethatisgreaterthan32bytes@12345": {
						"UserHostAddress": "192.168.2.132",
					},
				},
				"version": 1,
			},
			entry.Info,
			"info",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops, err := tc.config.Build(testutil.NewBuildContext(t))
			op := ops[0]
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			err = op.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			newEntry := entry.New()
			newEntry.Record = tc.inputRecord
			err = op.Process(context.Background(), newEntry)
			require.NoError(t, err)

			select {
			case e := <-fake.Received:
				require.Equal(t, tc.expectedRecord, e.Record)
				require.Equal(t, tc.expectedTimestamp, e.Timestamp)
				require.Equal(t, tc.expectedSeverity, e.Severity)
				require.Equal(t, tc.expectedSeverityText, e.SeverityText)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for entry to be processed")
			}
		})
	}
}
