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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func basicConfig() *ParserConfig {
	cfg := NewParserConfig("test_operator_id")
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func TestSyslogParser(t *testing.T) {
	cases, err := CreateCases(basicConfig)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			op, err := tc.Config.Build(testutil.Logger(t))
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			err = op.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			newEntry := tc.Input
			ots := newEntry.ObservedTimestamp

			err = op.Process(context.Background(), newEntry)
			require.NoError(t, err)

			select {
			case e := <-fake.Received:
				require.Equal(t, ots, e.ObservedTimestamp)
				require.Equal(t, tc.Expect, newEntry)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for entry to be processed")
			}
		})
	}
}

func TestSyslogParseRFC5424_SDNameTooLong(t *testing.T) {
	cfg := basicConfig()
	cfg.Protocol = RFC5424

	body := `<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [verylongsdnamethatisgreaterthan32bytes@12345 UserHostAddress="192.168.2.132"] my message`

	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	err = op.SetOutputs([]operator.Operator{fake})
	require.NoError(t, err)

	newEntry := entry.New()
	newEntry.Body = body
	err = op.Process(context.Background(), newEntry)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expecting a structured data element id (from 1 to max 32 US-ASCII characters")

	select {
	case e := <-fake.Received:
		require.Equal(t, body, e.Body)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be processed")
	}
}

func TestParserConfig(t *testing.T) {
	expect := NewParserConfig("test")
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
		var actual ParserConfig
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
		var actual ParserConfig
		err := yaml.Unmarshal([]byte(input), &actual)
		require.NoError(t, err)
		require.Equal(t, expect, &actual)
	})
}

func TestSyslogParserInvalidLocation(t *testing.T) {
	config := NewParserConfig("test")
	config.Location = "not_a_location"
	config.Protocol = RFC3164

	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load location "+config.Location)
}
