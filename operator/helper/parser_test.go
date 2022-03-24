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

package helper

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestParserConfigMissingBase(t *testing.T) {
	config := ParserConfig{}
	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required `type` field.")
}

func TestParserConfigInvalidTimeParser(t *testing.T) {
	cfg := NewParserConfig("test-id", "test-type")
	f := entry.NewBodyField("timestamp")
	cfg.TimeParser = &TimeParser{
		ParseFrom:  &f,
		Layout:     "",
		LayoutType: "strptime",
	}

	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required configuration parameter `layout`")
}

func TestParserConfigBuildValid(t *testing.T) {
	cfg := NewParserConfig("test-id", "test-type")
	f := entry.NewBodyField("timestamp")
	cfg.TimeParser = &TimeParser{
		ParseFrom:  &f,
		Layout:     "",
		LayoutType: "native",
	}
	_, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestParserMissingField(t *testing.T) {
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: WriterOperator{
				BasicOperator: BasicOperator{
					OperatorID:    "test-id",
					OperatorType:  "test-type",
					SugaredLogger: zaptest.NewLogger(t).Sugar(),
				},
			},
			OnError: DropOnError,
		},
		ParseFrom: entry.NewBodyField("test"),
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Entry is missing the expected parse_from field.")
}

func TestParserInvalidParseDrop(t *testing.T) {
	writer, fakeOut := writerWithFakeOut(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: *writer,
			OnError:        DropOnError,
		},
		ParseFrom: entry.NewBodyField(),
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, fmt.Errorf("parse failure")
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse failure")
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond)
}

func TestParserInvalidParseSend(t *testing.T) {
	writer, fakeOut := writerWithFakeOut(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: *writer,
			OnError:        SendOnError,
		},
		ParseFrom: entry.NewBodyField(),
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, fmt.Errorf("parse failure")
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse failure")
	fakeOut.ExpectEntry(t, testEntry)
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond)
}

func TestParserInvalidTimeParseDrop(t *testing.T) {
	writer, fakeOut := writerWithFakeOut(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: *writer,
			OnError:        DropOnError,
		},
		ParseFrom: entry.NewBodyField(),
		ParseTo:   entry.NewBodyField(),
		TimeParser: &TimeParser{
			ParseFrom: func() *entry.Field {
				f := entry.NewBodyField("missing-key")
				return &f
			}(),
		},
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.Error(t, err)
	require.Contains(t, err.Error(), "time parser: log entry does not have the expected parse_from field")
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond)
}

func TestParserInvalidTimeParseSend(t *testing.T) {
	writer, fakeOut := writerWithFakeOut(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: *writer,
			OnError:        SendOnError,
		},
		ParseFrom: entry.NewBodyField(),
		ParseTo:   entry.NewBodyField(),
		TimeParser: &TimeParser{
			ParseFrom: func() *entry.Field {
				f := entry.NewBodyField("missing-key")
				return &f
			}(),
		},
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.Error(t, err)
	require.Contains(t, err.Error(), "time parser: log entry does not have the expected parse_from field")
	fakeOut.ExpectEntry(t, testEntry)
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond)
}
func TestParserInvalidSeverityParseDrop(t *testing.T) {
	writer, fakeOut := writerWithFakeOut(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: *writer,
			OnError:        DropOnError,
		},
		SeverityParser: &SeverityParser{
			ParseFrom: entry.NewBodyField("missing-key"),
		},
		ParseFrom: entry.NewBodyField(),
		ParseTo:   entry.NewBodyField(),
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.Error(t, err)
	require.Contains(t, err.Error(), "severity parser: log entry does not have the expected parse_from field")
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond)
}

func TestParserInvalidTimeValidSeverityParse(t *testing.T) {
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: WriterOperator{
				BasicOperator: BasicOperator{
					OperatorID:    "test-id",
					OperatorType:  "test-type",
					SugaredLogger: testutil.Logger(t),
				},
			},
			OnError: DropOnError,
		},
		TimeParser: &TimeParser{
			ParseFrom: func() *entry.Field {
				f := entry.NewBodyField("missing-key")
				return &f
			}(),
		},
		SeverityParser: &SeverityParser{
			ParseFrom: entry.NewBodyField("severity"),
			Mapping: map[string]entry.Severity{
				"info": entry.Info,
			},
		},
		ParseFrom: entry.NewBodyField(),
		ParseTo:   entry.NewBodyField(),
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := testEntry.Set(entry.NewBodyField("severity"), "info")
	require.NoError(t, err)

	err = parser.ProcessWith(ctx, testEntry, parse)
	require.Error(t, err)
	require.Contains(t, err.Error(), "time parser: log entry does not have the expected parse_from field")

	// But, this should have been set anyways
	require.Equal(t, entry.Info, testEntry.Severity)
}

func TestParserValidTimeInvalidSeverityParse(t *testing.T) {
	// Hawaiian Standard Time
	hst, err := time.LoadLocation("HST")
	require.NoError(t, err)

	layout := "Mon Jan 2 15:04:05 MST 2006"
	sample := "Mon Dec 8 16:05:06 HST 2020"

	expected, err := time.ParseInLocation(layout, sample, hst)
	require.NoError(t, err)

	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: WriterOperator{
				BasicOperator: BasicOperator{
					OperatorID:    "test-id",
					OperatorType:  "test-type",
					SugaredLogger: testutil.Logger(t),
				},
			},
			OnError: DropOnError,
		},
		TimeParser: &TimeParser{
			ParseFrom: func() *entry.Field {
				f := entry.NewBodyField("timestamp")
				return &f
			}(),
			LayoutType: "gotime",
			Layout:     layout,
		},
		SeverityParser: &SeverityParser{
			ParseFrom: entry.NewBodyField("missing-key"),
		},
		ParseFrom: entry.NewBodyField(),
		ParseTo:   entry.NewBodyField(),
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err = testEntry.Set(entry.NewBodyField("timestamp"), sample)
	require.NoError(t, err)

	err = parser.ProcessWith(ctx, testEntry, parse)
	require.Error(t, err)
	require.Contains(t, err.Error(), "severity parser: log entry does not have the expected parse_from field")

	require.Equal(t, expected, testEntry.Timestamp)
}

func TestParserOutput(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)

	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			OnError: DropOnError,
			WriterOperator: WriterOperator{
				BasicOperator: BasicOperator{
					OperatorID:    "test-id",
					OperatorType:  "test-type",
					SugaredLogger: testutil.Logger(t),
				},
				OutputOperators: []operator.Operator{output},
			},
		},
		ParseFrom: entry.NewBodyField(),
		ParseTo:   entry.NewBodyField(),
	}
	parse := func(i interface{}) (interface{}, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.NoError(t, err)
	output.AssertCalled(t, "Process", mock.Anything, mock.Anything)
}

func TestParserPreserve(t *testing.T) {
	cases := []struct {
		name       string
		cfgMod     func(*ParserConfig)
		inputBody  interface{}
		outputBody interface{}
	}{
		{
			"NoPreserve",
			func(cfg *ParserConfig) {},
			"key:value",
			map[string]interface{}{"key": "value"},
		},
		{
			"PreserveToSubkey",
			func(cfg *ParserConfig) {
				dst := entry.NewBodyField("original")
				cfg.PreserveTo = &dst
			},
			"key:value",
			map[string]interface{}{"key": "value", "original": "key:value"},
		},
		{
			"PreserveToOverwrite",
			func(cfg *ParserConfig) {
				dst := entry.NewBodyField("key")
				cfg.PreserveTo = &dst
			},
			"key:value",
			map[string]interface{}{"key": "key:value"},
		},
		{
			"PreserveToRoot",
			func(cfg *ParserConfig) {
				dst := entry.NewBodyField()
				cfg.PreserveTo = &dst
			},
			"key:value",
			"key:value",
		},
		{
			"AlternativeParseFrom",
			func(cfg *ParserConfig) {
				dst := entry.NewBodyField("source")
				cfg.PreserveTo = &dst
				cfg.ParseFrom = dst
			},
			map[string]interface{}{"source": "key:value"},
			map[string]interface{}{"source": "key:value", "key": "value"},
		},
		{
			"AlternativeParseTo",
			func(cfg *ParserConfig) {
				dst := entry.NewBodyField("original")
				cfg.PreserveTo = &dst
				cfg.ParseTo = entry.NewBodyField("source_parsed")
			},
			"key:value",
			map[string]interface{}{
				"source_parsed": map[string]interface{}{
					"key": "value",
				},
				"original": "key:value",
			},
		},
	}

	parse := func(i interface{}) (interface{}, error) {
		split := strings.Split(i.(string), ":")
		return map[string]interface{}{split[0]: split[1]}, nil
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewParserConfig("test-id", "test-type")
			tc.cfgMod(&cfg)

			parser, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			e := entry.New()
			e.Body = tc.inputBody
			err = parser.ProcessWith(context.Background(), e, parse)
			require.NoError(t, err)

			require.Equal(t, tc.outputBody, e.Body)
		})
	}
}
func NewTestParserConfig() ParserConfig {
	except := NewParserConfig("parser_config", "test_type")
	except.ParseFrom = entry.NewBodyField("from")
	except.ParseTo = entry.NewBodyField("to")
	tp := NewTimeParser()
	sp := NewSeverityParserConfig()
	sp.Mapping = map[interface{}]interface{}{
		"info": "3xx",
		"warn": "4xx"}
	except.TimeParser = &tp
	except.SeverityParserConfig = &sp

	return except
}

func TestMapStructureDecodeParserConfigWithHook(t *testing.T) {
	except := NewTestParserConfig()
	input := map[string]interface{}{
		"id":         "parser_config",
		"type":       "test_type",
		"on_error":   "send",
		"parse_from": "body.from",
		"parse_to":   "body.to",
		"timestamp": map[string]interface{}{
			"layout_type": "strptime",
		},
		"severity": map[string]interface{}{
			"mapping": map[interface{}]interface{}{
				"info": "3xx",
				"warn": "4xx",
			},
		},
	}

	var actual ParserConfig
	dc := &mapstructure.DecoderConfig{Result: &actual, DecodeHook: JSONUnmarshalerHook()}
	ms, err := mapstructure.NewDecoder(dc)
	require.NoError(t, err)
	err = ms.Decode(input)
	require.NoError(t, err)
	require.Equal(t, except, actual)
}

func TestMapStructureDecodeParserConfig(t *testing.T) {
	except := NewTestParserConfig()
	input := map[string]interface{}{
		"id":         "parser_config",
		"type":       "test_type",
		"on_error":   "send",
		"parse_from": entry.NewBodyField("from"),
		"parse_to":   entry.NewBodyField("to"),
		"timestamp": map[string]interface{}{
			"layout_type": "strptime",
		},
		"severity": map[string]interface{}{
			"mapping": map[interface{}]interface{}{
				"info": "3xx",
				"warn": "4xx",
			},
		},
	}

	var actual ParserConfig
	err := mapstructure.Decode(input, &actual)
	require.NoError(t, err)
	require.Equal(t, except, actual)
}

func writerWithFakeOut(t *testing.T) (*WriterOperator, *testutil.FakeOutput) {
	fakeOut := testutil.NewFakeOutput(t)
	writer := &WriterOperator{
		BasicOperator: BasicOperator{
			OperatorID:    "test-id",
			OperatorType:  "test-type",
			SugaredLogger: testutil.Logger(t),
		},
		OutputIDs: []string{fakeOut.ID()},
	}
	writer.SetOutputs([]operator.Operator{fakeOut})
	return writer, fakeOut
}
