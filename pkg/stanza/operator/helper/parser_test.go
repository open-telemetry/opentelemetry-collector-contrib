// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestParserConfigMissingBase(t *testing.T) {
	config := ParserConfig{}
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.ErrorContains(t, err, "missing required `type` field.")
}

func TestParserConfigInvalidTimeParser(t *testing.T) {
	cfg := NewParserConfig("test-id", "test-type")
	f := entry.NewBodyField("timestamp")
	cfg.TimeParser = &TimeParser{
		ParseFrom:  &f,
		Layout:     "",
		LayoutType: "strptime",
	}

	set := componenttest.NewNopTelemetrySettings()
	_, err := cfg.Build(set)
	require.ErrorContains(t, err, "missing required configuration parameter `layout`")
}

func TestParserConfigBodyCollision(t *testing.T) {
	cfg := NewParserConfig("test-id", "test-type")
	cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField()}

	b := entry.NewAttributeField("message")
	cfg.BodyField = &b

	set := componenttest.NewNopTelemetrySettings()
	_, err := cfg.Build(set)
	require.ErrorContains(t, err, "`parse_to: body` not allowed when `body` is configured")
}

func TestParserConfigBuildValid(t *testing.T) {
	cfg := NewParserConfig("test-id", "test-type")

	timeField := entry.NewBodyField("timestamp")
	cfg.TimeParser = &TimeParser{
		ParseFrom:  &timeField,
		Layout:     "",
		LayoutType: "native",
	}

	sevField := entry.NewBodyField("timestamp")
	cfg.SeverityConfig = &SeverityConfig{
		ParseFrom: &sevField,
	}

	traceIDField := entry.NewBodyField("trace_id")
	spanIDField := entry.NewBodyField("span_id")
	traceFlagsField := entry.NewBodyField("trace_flags")
	cfg.TraceParser = &TraceParser{
		TraceID: &TraceIDConfig{
			ParseFrom: &traceIDField,
		},
		SpanID: &SpanIDConfig{
			ParseFrom: &spanIDField,
		},
		TraceFlags: &TraceFlagsConfig{
			ParseFrom: &traceFlagsField,
		},
	}

	scopeNameField := entry.NewBodyField("logger")
	cfg.ScopeNameParser = &ScopeNameParser{
		ParseFrom: scopeNameField,
	}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	require.NotNil(t, op.TimeParser)
	require.NotNil(t, op.SeverityParser)
	require.NotNil(t, op.TraceParser)
	require.NotNil(t, op.ScopeNameParser)
}

func TestParserMissingField(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: WriterOperator{
				BasicOperator: BasicOperator{
					OperatorID:   "test-id",
					OperatorType: "test-type",
					set:          set,
				},
			},
			OnError: DropOnError,
		},
		ParseFrom: entry.NewBodyField("test"),
	}
	parse := func(i any) (any, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.ErrorContains(t, err, "Entry is missing the expected parse_from field.")
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
	parse := func(i any) (any, error) {
		return i, fmt.Errorf("parse failure")
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.ErrorContains(t, err, "parse failure")
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond)
}

func TestParserInvalidParseDropQuiet(t *testing.T) {
	writer, fakeOut := writerWithFakeOut(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: *writer,
			OnError:        DropOnErrorQuiet,
		},
		ParseFrom: entry.NewBodyField(),
	}
	parse := func(i any) (any, error) {
		return i, fmt.Errorf("parse failure")
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.NoError(t, err, "error should be silent")
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond) // Entry should be dropped
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
	parse := func(i any) (any, error) {
		return i, fmt.Errorf("parse failure")
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.ErrorContains(t, err, "parse failure")
	fakeOut.ExpectEntry(t, testEntry)
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond)
}

func TestParserInvalidParseSendQuiet(t *testing.T) {
	writer, fakeOut := writerWithFakeOut(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: *writer,
			OnError:        SendOnErrorQuiet,
		},
		ParseFrom: entry.NewBodyField(),
	}
	parse := func(i any) (any, error) {
		return i, fmt.Errorf("parse failure")
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.NoError(t, err, "error should be silent")
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
	parse := func(i any) (any, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.ErrorContains(t, err, "time parser: log entry does not have the expected parse_from field")
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
	parse := func(i any) (any, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.ErrorContains(t, err, "time parser: log entry does not have the expected parse_from field")
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
	parse := func(i any) (any, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.ErrorContains(t, err, "severity parser: log entry does not have the expected parse_from field")
	fakeOut.ExpectNoEntry(t, 100*time.Millisecond)
}

func TestParserInvalidTimeValidSeverityParse(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: WriterOperator{
				BasicOperator: BasicOperator{
					OperatorID:   "test-id",
					OperatorType: "test-type",
					set:          set,
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
	parse := func(i any) (any, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := testEntry.Set(entry.NewBodyField("severity"), "info")
	require.NoError(t, err)

	err = parser.ProcessWith(ctx, testEntry, parse)
	require.ErrorContains(t, err, "time parser: log entry does not have the expected parse_from field")

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

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)

	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			WriterOperator: WriterOperator{
				BasicOperator: BasicOperator{
					OperatorID:   "test-id",
					OperatorType: "test-type",
					set:          set,
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
	parse := func(i any) (any, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err = testEntry.Set(entry.NewBodyField("timestamp"), sample)
	require.NoError(t, err)

	err = parser.ProcessWith(ctx, testEntry, parse)
	require.ErrorContains(t, err, "severity parser: log entry does not have the expected parse_from field")

	require.Equal(t, expected, testEntry.Timestamp)
}

func TestParserOutput(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)

	parser := ParserOperator{
		TransformerOperator: TransformerOperator{
			OnError: DropOnError,
			WriterOperator: WriterOperator{
				BasicOperator: BasicOperator{
					OperatorID:   "test-id",
					OperatorType: "test-type",
					set:          set,
				},
				OutputOperators: []operator.Operator{output},
			},
		},
		ParseFrom: entry.NewBodyField(),
		ParseTo:   entry.NewBodyField(),
	}
	parse := func(i any) (any, error) {
		return i, nil
	}
	ctx := context.Background()
	testEntry := entry.New()
	err := parser.ProcessWith(ctx, testEntry, parse)
	require.NoError(t, err)
	output.AssertCalled(t, "Process", mock.Anything, mock.Anything)
}

func TestParserFields(t *testing.T) {
	keyValue := "key:value"
	now := time.Now()
	cases := []struct {
		name   string
		cfgMod func(*ParserConfig)
		input  func() *entry.Entry
		output func() *entry.Entry
	}{
		{
			"ParseToBodyRoot",
			func(cfg *ParserConfig) {
				cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = map[string]any{
					"key": "value",
				}
				return e
			},
		},
		{
			"ParseToAttributesRoot",
			func(cfg *ParserConfig) {
				cfg.ParseTo = entry.RootableField{Field: entry.NewAttributeField()}
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				e.Attributes = map[string]any{
					"key": "value",
				}
				return e
			},
		},
		{
			"ParseToResourceRoot",
			func(cfg *ParserConfig) {
				cfg.ParseTo = entry.RootableField{Field: entry.NewResourceField()}
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				e.Resource = map[string]any{
					"key": "value",
				}
				return e
			},
		},
		{
			"ParseToBodyField",
			func(cfg *ParserConfig) {
				cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField("one", "two")}
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"key": "value",
						},
					},
				}
				return e
			},
		},
		{
			"ParseToAttributeField",
			func(cfg *ParserConfig) {
				cfg.ParseTo = entry.RootableField{Field: entry.NewAttributeField("one", "two")}
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				e.Attributes = map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"key": "value",
						},
					},
				}
				return e
			},
		},
		{
			"ParseToResourceField",
			func(cfg *ParserConfig) {
				cfg.ParseTo = entry.RootableField{Field: entry.NewResourceField("one", "two")}
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				e.Resource = map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"key": "value",
						},
					},
				}
				return e
			},
		},
		{
			"ParseAndSetBody",
			func(cfg *ParserConfig) {
				b := entry.NewAttributeField("key")
				cfg.BodyField = &b
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = keyValue
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Attributes = map[string]any{
					"key": "value",
				}
				e.Body = "value"
				return e
			},
		},
		{
			"ParseFromBodyField",
			func(cfg *ParserConfig) {
				cfg.ParseFrom = entry.NewBodyField("one", "two")
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = map[string]any{
					"one": map[string]any{
						"two": keyValue,
					},
				}
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = map[string]any{
					"one": map[string]any{
						"two": keyValue,
					},
				}
				e.Attributes = map[string]any{
					"key": "value",
				}
				return e
			},
		},
		{
			"ParseFromAttributeField",
			func(cfg *ParserConfig) {
				cfg.ParseFrom = entry.NewAttributeField("one", "two")
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Attributes = map[string]any{
					"one": map[string]any{
						"two": keyValue,
					},
				}
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Attributes = map[string]any{
					"key": "value",
					"one": map[string]any{
						"two": keyValue,
					},
				}
				return e
			},
		},
		{
			"ParseFromResourceField",
			func(cfg *ParserConfig) {
				cfg.ParseFrom = entry.NewResourceField("one", "two")
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Resource = map[string]any{
					"one": map[string]any{
						"two": keyValue,
					},
				}
				return e
			},
			func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Attributes = map[string]any{
					"key": "value",
				}
				e.Resource = map[string]any{
					"one": map[string]any{
						"two": keyValue,
					},
				}
				return e
			},
		},
	}

	parse := func(i any) (any, error) {
		split := strings.Split(i.(string), ":")
		return map[string]any{split[0]: split[1]}, nil
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewParserConfig("test-id", "test-type")
			tc.cfgMod(&cfg)

			set := componenttest.NewNopTelemetrySettings()
			parser, err := cfg.Build(set)
			require.NoError(t, err)

			e := tc.input()
			err = parser.ProcessWith(context.Background(), e, parse)

			require.NoError(t, err)
			require.Equal(t, tc.output(), e)
		})
	}
}

func NewTestParserConfig() ParserConfig {
	expect := NewParserConfig("parser_config", "test_type")
	expect.ParseFrom = entry.NewBodyField("from")
	expect.ParseTo = entry.RootableField{Field: entry.NewBodyField("to")}
	tp := NewTimeParser()
	expect.TimeParser = &tp

	sp := NewSeverityConfig()
	sp.Mapping = map[string]any{
		"info": "3xx",
		"warn": "4xx",
	}
	expect.SeverityConfig = &sp

	lnp := NewScopeNameParser()
	lnp.ParseFrom = entry.NewBodyField("logger")
	expect.ScopeNameParser = &lnp
	return expect
}

func writerWithFakeOut(t *testing.T) (*WriterOperator, *testutil.FakeOutput) {
	fakeOut := testutil.NewFakeOutput(t)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	writer := &WriterOperator{
		BasicOperator: BasicOperator{
			OperatorID:   "test-id",
			OperatorType: "test-type",
			set:          set,
		},
		OutputIDs: []string{fakeOut.ID()},
	}
	require.NoError(t, writer.SetOutputs([]operator.Operator{fakeOut}))
	return writer, fakeOut
}
