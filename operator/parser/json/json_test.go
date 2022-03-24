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

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func newTestParser(t *testing.T) *JSONParser {
	config := NewJSONParserConfig("test")
	op, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*JSONParser)
}

func TestJSONParserConfigBuild(t *testing.T) {
	config := NewJSONParserConfig("test")
	op, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.IsType(t, &JSONParser{}, op)
}

func TestJSONParserConfigBuildFailure(t *testing.T) {
	config := NewJSONParserConfig("test")
	config.OnError = "invalid_on_error"
	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestJSONParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error found in #1 byte")
}

func TestJSONParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []uint8 cannot be parsed as JSON")
}

func TestJSONParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []int cannot be parsed as JSON")
}

func NewFakeJSONOperator() (*JSONParser, *testutil.Operator) {
	mock := testutil.Operator{}
	logger, _ := zap.NewProduction()
	return &JSONParser{
		ParserOperator: helper.ParserOperator{
			TransformerOperator: helper.TransformerOperator{
				WriterOperator: helper.WriterOperator{
					BasicOperator: helper.BasicOperator{
						OperatorID:    "test",
						OperatorType:  "json_parser",
						SugaredLogger: logger.Sugar(),
					},
					OutputOperators: []operator.Operator{&mock},
				},
			},
			ParseFrom: entry.NewBodyField("testfield"),
			ParseTo:   entry.NewBodyField("testparsed"),
		},
		json: jsoniter.ConfigFastest,
	}, &mock
}

func TestJSONImplementations(t *testing.T) {
	require.Implements(t, (*operator.Operator)(nil), new(JSONParser))
}

func TestJSONParser(t *testing.T) {
	cases := []struct {
		name          string
		inputBody     map[string]interface{}
		outputBody    map[string]interface{}
		errorExpected bool
	}{
		{
			"simple",
			map[string]interface{}{
				"testfield": `{}`,
			},
			map[string]interface{}{
				"testparsed": map[string]interface{}{},
			},
			false,
		},
		{
			"nested",
			map[string]interface{}{
				"testfield": `{"superkey":"superval"}`,
			},
			map[string]interface{}{
				"testparsed": map[string]interface{}{
					"superkey": "superval",
				},
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := entry.New()
			input.Body = tc.inputBody
			output := entry.New()
			output.Body = tc.outputBody

			parser, mockOutput := NewFakeJSONOperator()
			mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				e := args[1].(*entry.Entry)
				require.Equal(t, tc.outputBody, e.Body)
			}).Return(nil)

			err := parser.Process(context.Background(), input)
			require.NoError(t, err)
		})
	}
}

func TestJSONParserWithEmbeddedTimeParser(t *testing.T) {
	testTime := time.Unix(1136214245, 0)

	cases := []struct {
		name          string
		inputBody     map[string]interface{}
		outputBody    map[string]interface{}
		errorExpected bool
		preserveTo    *entry.Field
	}{
		{
			"simple",
			map[string]interface{}{
				"testfield": `{"timestamp":1136214245}`,
			},
			map[string]interface{}{
				"testparsed": map[string]interface{}{},
			},
			false,
			nil,
		},
		{
			"preserve",
			map[string]interface{}{
				"testfield": `{"timestamp":"1136214245"}`,
			},
			map[string]interface{}{
				"testparsed":         map[string]interface{}{},
				"original_timestamp": "1136214245",
			},
			false,
			func() *entry.Field {
				f := entry.NewBodyField("original_timestamp")
				return &f
			}(),
		},
		{
			"nested",
			map[string]interface{}{
				"testfield": `{"superkey":"superval","timestamp":1136214245.123}`,
			},
			map[string]interface{}{
				"testparsed": map[string]interface{}{
					"superkey": "superval",
				},
			},
			false,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := entry.New()
			input.Body = tc.inputBody

			output := entry.New()
			output.Body = tc.outputBody

			parser, mockOutput := NewFakeJSONOperator()
			parseFrom := entry.NewBodyField("testparsed", "timestamp")
			parser.ParserOperator.TimeParser = &helper.TimeParser{
				ParseFrom:  &parseFrom,
				LayoutType: "epoch",
				Layout:     "s",
				PreserveTo: tc.preserveTo,
			}
			mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				e := args[1].(*entry.Entry)
				require.Equal(t, tc.outputBody, e.Body)
				require.Equal(t, testTime, e.Timestamp)
			}).Return(nil)

			err := parser.Process(context.Background(), input)
			require.NoError(t, err)
		})
	}
}

func TestJSONParserWithEmbeddedScopeNameParser(t *testing.T) {
	cases := []struct {
		name          string
		inputBody     map[string]interface{}
		outputBody    map[string]interface{}
		errorExpected bool
		preserveTo    *entry.Field
	}{
		{
			"simple",
			map[string]interface{}{
				"testfield": `{"logger_name":"logger"}`,
			},
			map[string]interface{}{
				"testparsed": map[string]interface{}{},
			},
			false,
			nil,
		},
		{
			"preserve",
			map[string]interface{}{
				"testfield": `{"logger_name":"logger"}`,
			},
			map[string]interface{}{
				"testparsed":           map[string]interface{}{},
				"original_logger_name": "logger",
			},
			false,
			func() *entry.Field {
				f := entry.NewBodyField("original_logger_name")
				return &f
			}(),
		},
		{
			"nested",
			map[string]interface{}{
				"testfield": `{"superkey":"superval","logger_name":"logger"}`,
			},
			map[string]interface{}{
				"testparsed": map[string]interface{}{
					"superkey": "superval",
				},
			},
			false,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := entry.New()
			input.Body = tc.inputBody

			output := entry.New()
			output.Body = tc.outputBody

			parser, mockOutput := NewFakeJSONOperator()
			parseFrom := entry.NewBodyField("testparsed", "logger_name")
			parser.ParserOperator.ScopeNameParser = &helper.ScopeNameParser{
				ParseFrom:  parseFrom,
				PreserveTo: tc.preserveTo,
			}
			mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				e := args[1].(*entry.Entry)
				require.Equal(t, tc.outputBody, e.Body)
			}).Return(nil)

			err := parser.Process(context.Background(), input)
			require.NoError(t, err)
		})
	}
}

func TestJsonParserConfig(t *testing.T) {
	expect := NewJSONParserConfig("test")
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
		var actual JSONParserConfig
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
		var actual JSONParserConfig
		err := yaml.Unmarshal([]byte(input), &actual)
		require.NoError(t, err)
		require.Equal(t, expect, &actual)
	})
}
