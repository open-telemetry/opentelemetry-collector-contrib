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
			op, err := tc.Config.Build(testutil.Logger(t))
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			err = op.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			newEntry := entry.New()
			ots := newEntry.ObservedTimestamp
			newEntry.Body = tc.InputBody
			err = op.Process(context.Background(), newEntry)
			require.NoError(t, err)

			select {
			case e := <-fake.Received:
				require.Equal(t, ots, e.ObservedTimestamp)
				require.Equal(t, tc.ExpectedBody, e.Body)
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
	expect.Protocol = RFC3164
	expect.ParseFrom = entry.NewBodyField("from")
	expect.ParseTo = entry.NewBodyField("to")

	t.Run("mapstructure", func(t *testing.T) {
		input := map[string]interface{}{
			"id":         "test",
			"type":       "syslog_parser",
			"protocol":   RFC3164,
			"parse_from": "body.from",
			"parse_to":   "body.to",
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
protocol: rfc3164
parse_from: body.from
parse_to: body.to`
		var actual SyslogParserConfig
		err := yaml.Unmarshal([]byte(input), &actual)
		require.NoError(t, err)
		require.Equal(t, expect, &actual)
	})
}

func TestSyslogParserInvalidLocation(t *testing.T) {
	config := NewSyslogParserConfig("test")
	config.Location = "not_a_location"
	config.Protocol = RFC3164

	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load location "+config.Location)
}
