// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package jsonarray

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	commontestutil "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T) *Parser {
	defer commontestutil.SetFeatureGateForTest(t, jsonArrayParserFeatureGate, true)()

	cfg := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	return op.(*Parser)
}

func TestParserBuildFailure(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, jsonArrayParserFeatureGate, true)()

	cfg := NewConfigWithID("test")
	cfg.OnError = "invalid_on_error"
	set := componenttest.NewNopTelemetrySettings()
	_, err := cfg.Build(set)
	require.ErrorContains(t, err, "invalid `on_error` field")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.ErrorContains(t, err, "type '[]int' cannot be parsed as json array")
}

func TestParserByteFailureHeadersMismatch(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, jsonArrayParserFeatureGate, true)()

	cfg := NewConfigWithID("test")
	cfg.Header = "name,sev,msg"
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	parser := op.(*Parser)
	_, err = parser.parse("[\"stanza\",\"INFO\",\"started agent\", 42, true]")
	require.ErrorContains(t, err, "wrong number of fields: expected 3, found 5")
}

func TestParserJarray(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, jsonArrayParserFeatureGate, true)()

	cases := []struct {
		name             string
		configure        func(*Config)
		inputEntries     []entry.Entry
		expectedEntries  []entry.Entry
		expectBuildErr   bool
		expectProcessErr bool
	}{
		{
			"basic",
			func(p *Config) {
				p.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
				},
			},
			[]entry.Entry{
				{
					Body: []any{"stanza", "INFO", "started agent", int64(42), true},
				},
			},
			false,
			false,
		},
		{
			"basic-no-parse_to-fail",
			func(_ *Config) {
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
				},
			},
			[]entry.Entry{
				{
					Body: []any{"stanza", "INFO", "started agent", int64(42), true},
				},
			},
			false,
			true,
		},
		{
			"parse-to-attributes",
			func(p *Config) {
				p.ParseTo = entry.RootableField{Field: entry.NewAttributeField("output")}
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
				},
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
					Attributes: map[string]any{
						"output": []any{"stanza", "INFO", "started agent", int64(42), true},
					},
				},
			},
			false,
			false,
		},
		{
			"parse-to-and-from-attributes",
			func(p *Config) {
				p.ParseTo = entry.RootableField{Field: entry.NewAttributeField("output")}
				p.ParseFrom = entry.NewAttributeField("input")
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
					Attributes: map[string]any{
						"input": "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
					},
				},
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
					Attributes: map[string]any{
						"input":  "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
						"output": []any{"stanza", "INFO", "started agent", int64(42), true},
					},
				},
			},
			false,
			false,
		},
		{
			"nested-object",
			func(p *Config) {
				p.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\", 42, {\"city\": \"New York\", \"zip\": 10001}]",
				},
			},
			[]entry.Entry{
				{
					Body: []any{"stanza", int64(42), "{\"city\":\"New York\",\"zip\":10001}"},
				},
			},
			false,
			false,
		},
		{
			"basic-multiple-static-bodies",
			func(p *Config) {
				p.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
				},
				{
					Body: "[\"stanza\",\"ERROR\",\"agent killed\", 9999999, false]",
				},
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 0, null]",
				},
			},
			[]entry.Entry{
				{
					Body: []any{"stanza", "INFO", "started agent", int64(42), true},
				},
				{
					Body: []any{"stanza", "ERROR", "agent killed", int64(9999999), false},
				},
				{
					Body: []any{"stanza", "INFO", "started agent", int64(0), nil},
				},
			},
			false,
			false,
		},
		{
			"comma in quotes",
			func(p *Config) {
				p.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"Evergreen,49508\",1,\"555-5555\",\"agent\"]",
				},
			},
			[]entry.Entry{
				{
					Body: []any{"stanza", "Evergreen,49508", int64(1), "555-5555", "agent"},
				},
			},
			false,
			false,
		},
		{
			"parse-as-attributes-with-header",
			func(p *Config) {
				p.ParseTo = entry.RootableField{Field: entry.NewAttributeField()}
				p.Header = "origin,sev,message,count,isBool"
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
				},
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
					Attributes: map[string]any{
						"origin":  "stanza",
						"sev":     "INFO",
						"message": "started agent",
						"count":   int64(42),
						"isBool":  true,
					},
				},
			},
			false,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			if tc.expectBuildErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

			ots := time.Now()
			for i := range tc.inputEntries {
				inputEntry := tc.inputEntries[i]
				inputEntry.ObservedTimestamp = ots
				err = op.Process(context.Background(), &inputEntry)
				if tc.expectProcessErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)

				expectedEntry := tc.expectedEntries[i]
				expectedEntry.ObservedTimestamp = ots
				fake.ExpectEntry(t, &expectedEntry)
			}
		})
	}
}

func TestParserJarrayMultiline(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, jsonArrayParserFeatureGate, true)()

	cases := []struct {
		name     string
		input    string
		expected []any
	}{
		{
			"no_newlines",
			"[\"aaaa\",\"bbbb\",12,true,\"eeee\"]",
			[]any{"aaaa", "bbbb", int64(12), true, "eeee"},
		},
		{
			"first_field",
			"[\"aa\naa\",\"bbbb\",12,true,\"eeee\"]",
			[]any{"aa\naa", "bbbb", int64(12), true, "eeee"},
		},
		{
			"middle_field",
			"[\"aaaa\",\"bb\nbb\",12,true,\"eeee\"]",
			[]any{"aaaa", "bb\nbb", int64(12), true, "eeee"},
		},
		{
			"last_field",
			"[\"aaaa\",\"bbbb\",12,true,\"e\neee\"]",
			[]any{"aaaa", "bbbb", int64(12), true, "e\neee"},
		},
		{
			"multiple_fields",
			"[\"aaaa\",\"bb\nbb\",12,true,\"e\neee\"]",
			[]any{"aaaa", "bb\nbb", int64(12), true, "e\neee"},
		},
		{
			"multiple_first_field",
			"[\"a\na\na\na\",\"bbbb\",\"cccc\",\"dddd\",\"eeee\"]",
			[]any{"a\na\na\na", "bbbb", "cccc", "dddd", "eeee"},
		},
		{
			"multiple_middle_field",
			"[\"aaaa\",\"bbbb\",\"c\nc\nc\nc\",\"dddd\",\"eeee\"]",
			[]any{"aaaa", "bbbb", "c\nc\nc\nc", "dddd", "eeee"},
		},
		{
			"multiple_last_field",
			"[\"aaaa\",\"bbbb\",\"cccc\",\"dddd\",\"e\ne\ne\ne\"]",
			[]any{"aaaa", "bbbb", "cccc", "dddd", "e\ne\ne\ne"},
		},
		{
			"leading_newline",
			"[\"\naaaa\",\"bbbb\",\"cccc\",\"dddd\",\"eeee\"]",
			[]any{"\naaaa", "bbbb", "cccc", "dddd", "eeee"},
		},
		{
			"trailing_newline",
			"[\"aaaa\",\"bbbb\",\"cccc\",\"dddd\",\"eeee\n\"]",
			[]any{"aaaa", "bbbb", "cccc", "dddd", "eeee\n"},
		},
		{
			"leading_newline_field",
			"[\"aaaa\",\"\nbbbb\",\"\ncccc\",\"\ndddd\",\"eeee\"]",
			[]any{"aaaa", "\nbbbb", "\ncccc", "\ndddd", "eeee"},
		},
		{
			"trailing_newline_field",
			"[\"aaaa\",\"bbbb\n\",\"cccc\n\",\"dddd\n\",\"eeee\"]",
			[]any{"aaaa", "bbbb\n", "cccc\n", "dddd\n", "eeee"},
		},
		{
			"empty_lines_unquoted",
			"[\"aa\naa\",\"bbbb\",\"c\nccc\",\"dddd\",\"eee\ne\"]",
			[]any{"aa\naa", "bbbb", "c\nccc", "dddd", "eee\ne"},
		},
		{
			"literal_return",
			`["aaaa","bb
bb","cccc","dd
dd","eeee"]`,
			[]any{"aaaa", "bb\nbb", "cccc", "dd\ndd", "eeee"},
		},
		{
			"return_in_quotes",
			"[\"aaaa\",\"bbbb\",\"cc\ncc\",\"dddd\",\"eeee\"]",
			[]any{"aaaa", "bbbb", "cc\ncc", "dddd", "eeee"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
			cfg.OutputIDs = []string{"fake"}

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

			entry := entry.New()
			entry.Body = tc.input
			err = op.Process(context.Background(), entry)
			require.NoError(t, err)
			fake.ExpectBody(t, tc.expected)
			fake.ExpectNoEntry(t, 100*time.Millisecond)
		})
	}
}

func TestBuildParserJarray(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, jsonArrayParserFeatureGate, true)()

	newBasicParser := func() *Config {
		cfg := NewConfigWithID("test")
		cfg.OutputIDs = []string{"test"}
		return cfg
	}

	t.Run("BasicConfig", func(t *testing.T) {
		c := newBasicParser()
		set := componenttest.NewNopTelemetrySettings()
		_, err := c.Build(set)
		require.NoError(t, err)
	})
}
