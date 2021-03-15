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

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestSyslogParser(t *testing.T) {
	basicConfig := func() *SyslogParserConfig {
		cfg := NewSyslogParserConfig("test_operator_id")
		cfg.OutputIDs = []string{"fake"}
		return cfg
	}

	cases, err := CreateCases(basicConfig)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ops, err := tc.Config.Build(testutil.NewBuildContext(t))
			op := ops[0]
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			err = op.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			newEntry := entry.New()
			newEntry.Record = tc.InputRecord
			err = op.Process(context.Background(), newEntry)
			require.NoError(t, err)

			select {
			case e := <-fake.Received:
				require.Equal(t, tc.ExpectedRecord, e.Record)
				require.Equal(t, tc.ExpectedTimestamp, e.Timestamp)
				require.Equal(t, tc.ExpectedSeverity, e.Severity)
				require.Equal(t, tc.ExpectedSeverityText, e.SeverityText)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for entry to be processed")
			}
		})
	}
}

func TestSyslogParserConfig(t *testing.T) {
	expect := NewSyslogParserConfig("test")
	expect.Protocol = "rfc3164"
	expect.ParseFrom = entry.NewRecordField("from")
	expect.ParseTo = entry.NewRecordField("to")

	t.Run("mapstructure", func(t *testing.T) {
		input := map[string]interface{}{
			"id":         "test",
			"type":       "syslog_parser",
			"protocol":   "rfc3164",
			"parse_from": "$.from",
			"parse_to":   "$.to",
			"on_error":   "send",
		}
		var actual SyslogParserConfig
		err := helper.UnmarshalMapstructure(input, &actual)
		require.NoError(t, err)
		require.Equal(t, expect, &actual)
	})

	t.Run("yaml", func(t *testing.T) {
		input := `
type: syslog_parser
id: test
on_error: "send"
protocol: "rfc3164"
parse_from: $.from
parse_to: $.to`
		var actual SyslogParserConfig
		err := yaml.Unmarshal([]byte(input), &actual)
		require.NoError(t, err)
		require.Equal(t, expect, &actual)
	})
}
