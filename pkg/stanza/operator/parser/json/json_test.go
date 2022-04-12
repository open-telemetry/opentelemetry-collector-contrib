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

package json

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

func newTestParser(t *testing.T) *Parser {
	config := NewParserConfig("test")
	op, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*Parser)
}

func TestParserConfigBuild(t *testing.T) {
	config := NewParserConfig("test")
	op, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.IsType(t, &Parser{}, op)
}

func TestParserConfigBuildFailure(t *testing.T) {
	config := NewParserConfig("test")
	config.OnError = "invalid_on_error"
	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error found in #1 byte")
}

func TestParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []uint8 cannot be parsed as JSON")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []int cannot be parsed as JSON")
}

func TestJSONImplementations(t *testing.T) {
	require.Implements(t, (*operator.Operator)(nil), new(Parser))
}

func TestParser(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*ParserConfig)
		input     *entry.Entry
		expect    *entry.Entry
	}{
		{
			"simple",
			func(p *ParserConfig) {},
			&entry.Entry{
				Body: `{}`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{},
				Body:       `{}`,
			},
		},
		{
			"nested",
			func(p *ParserConfig) {},
			&entry.Entry{
				Body: `{"superkey":"superval"}`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"superkey": "superval",
				},
				Body: `{"superkey":"superval"}`,
			},
		},
		{
			"with_timestamp",
			func(p *ParserConfig) {
				parseFrom := entry.NewAttributeField("timestamp")
				p.TimeParser = &helper.TimeParser{
					ParseFrom:  &parseFrom,
					LayoutType: "epoch",
					Layout:     "s",
				}
			},
			&entry.Entry{
				Body: `{"superkey":"superval","timestamp":1136214245}`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"superkey": "superval",
				},
				Body:      `{"superkey":"superval","timestamp":1136214245}`,
				Timestamp: time.Unix(1136214245, 0),
			},
		},
		{
			"with_scope",
			func(p *ParserConfig) {
				p.ScopeNameParser = &helper.ScopeNameParser{
					ParseFrom: entry.NewAttributeField("logger_name"),
				}
			},
			&entry.Entry{
				Body: `{"superkey":"superval","logger_name":"logger"}`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"superkey": "superval",
				},
				Body:      `{"superkey":"superval","logger_name":"logger"}`,
				ScopeName: "logger",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewParserConfig("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			op.SetOutputs([]operator.Operator{fake})

			ots := time.Now()
			tc.input.ObservedTimestamp = ots
			tc.expect.ObservedTimestamp = ots

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)
			fake.ExpectEntry(t, tc.expect)
		})
	}
}

func TestUnmarshalConfig(t *testing.T) {
	expect := NewParserConfig("test")
	expect.ParseFrom = entry.NewBodyField("from")
	expect.ParseTo = entry.NewBodyField("to")

	t.Run("mapstructure", func(t *testing.T) {
		input := map[string]interface{}{
			"id":         "test",
			"type":       "json_parser",
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
		input := `type: json_parser
id: test
on_error: "send"
parse_from: body.from
parse_to: body.to`
		var actual ParserConfig
		err := yaml.Unmarshal([]byte(input), &actual)
		require.NoError(t, err)
		require.Equal(t, expect, &actual)
	})
}
