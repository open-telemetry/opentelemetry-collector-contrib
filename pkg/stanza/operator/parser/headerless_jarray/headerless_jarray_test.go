// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package headerless_jarray

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

var testHeader = "name,sev,msg,count,isBool"

func newTestParser(t *testing.T) *Parser {
	cfg := NewConfigWithID("test")
	cfg.Header = testHeader
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*Parser)
}

func TestParserBuildFailure(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.OnError = "invalid_on_error"
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestParserBuildFailureBadHeaderConfig(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.Header = "testheader"
	cfg.HeaderAttribute = "testheader"
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "only one header parameter can be set: 'header' or 'header_attribute'")
}

func TestParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("[\"invalid\"]"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong number of fields: expected 5, found 1")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("[\"invalid\"]")
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong number of fields: expected 5, found 1")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as jarray")
}

func TestParserJarray(t *testing.T) {
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
				p.Header = testHeader
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
				},
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"sev":    "INFO",
						"msg":    "started agent",
						"count":  int64(42),
						"isBool": true,
					},
				},
			},
			false,
			false,
		},
		{
			"nested-object",
			func(p *Config) {
				p.Header = "name,age,nested"
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\", 42, {\"city\": \"New York\", \"zip\": 10001}]",
				},
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\", 42, {\"city\": \"New York\", \"zip\": 10001}]",
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    int64(42),
						"nested": "{\"city\":\"New York\",\"zip\":10001}",
					},
				},
			},
			false,
			false,
		},
		{
			"basic-multiple-static-bodies",
			func(p *Config) {
				p.Header = testHeader
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
					Body: "[\"stanza\",\"INFO\",\"started agent\", 42, true]",
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"sev":    "INFO",
						"msg":    "started agent",
						"count":  int64(42),
						"isBool": true,
					},
				},
				{
					Body: "[\"stanza\",\"ERROR\",\"agent killed\", 9999999, false]",
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"sev":    "ERROR",
						"msg":    "agent killed",
						"count":  int64(9999999),
						"isBool": false,
					},
				},
				{
					Body: "[\"stanza\",\"INFO\",\"started agent\", 0, null]",
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"sev":    "INFO",
						"msg":    "started agent",
						"count":  int64(0),
						"isBool": nil,
					},
				},
			},
			false,
			false,
		},
		{
			"advanced",
			func(p *Config) {
				p.Header = "name;address;age;phone;position"
				p.HeaderDelimiter = ";"
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"Evergreen\",1,\"555-5555\",\"agent\"]",
				},
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"Evergreen\",1,\"555-5555\",\"agent\"]",
					Attributes: map[string]interface{}{
						"name":     "stanza",
						"address":  "Evergreen",
						"age":      int64(1),
						"phone":    "555-5555",
						"position": "agent",
					},
				},
			},
			false,
			false,
		},
		{
			"dynamic-fields",
			func(p *Config) {
				p.HeaderAttribute = "Fields"
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name,age,height,number",
					},
					Body: "[\"stanza dev\",1,400,\"555-555-5555\"]",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name,age,height,number",
						"name":   "stanza dev",
						"age":    int64(1),
						"height": int64(400),
						"number": "555-555-5555",
					},
					Body: "[\"stanza dev\",1,400,\"555-555-5555\"]",
				},
			},
			false,
			false,
		},
		{
			"dynamic-fields-header-delimiter",
			func(p *Config) {
				p.HeaderAttribute = "Fields"
				p.HeaderDelimiter = "|"
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name|age|height|number",
					},
					Body: "[\"stanza dev\",1,400,\"555-555-5555\"]",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name|age|height|number",
						"name":   "stanza dev",
						"age":    int64(1),
						"height": int64(400),
						"number": "555-555-5555",
					},
					Body: "[\"stanza dev\",1,400,\"555-555-5555\"]",
				},
			},
			false,
			false,
		},
		{
			"dynamic-fields-multiple-entries",
			func(p *Config) {
				p.HeaderAttribute = "Fields"
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name,age,height,number",
					},
					Body: "[\"stanza dev\",1,400,\"555-555-5555\"]",
				},
				{
					Attributes: map[string]interface{}{
						"Fields": "x,y",
					},
					Body: "[\"000100\",2]",
				},
				{
					Attributes: map[string]interface{}{
						"Fields": "a,b,c,d,e,f",
					},
					Body: "[1,2,3,4,5,6]",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name,age,height,number",
						"name":   "stanza dev",
						"age":    int64(1),
						"height": int64(400),
						"number": "555-555-5555",
					},
					Body: "[\"stanza dev\",1,400,\"555-555-5555\"]",
				},
				{
					Attributes: map[string]interface{}{
						"Fields": "x,y",
						"x":      "000100",
						"y":      int64(2),
					},
					Body: "[\"000100\",2]",
				},
				{
					Attributes: map[string]interface{}{
						"Fields": "a,b,c,d,e,f",
						"a":      int64(1),
						"b":      int64(2),
						"c":      int64(3),
						"d":      int64(4),
						"e":      int64(5),
						"f":      int64(6),
					},
					Body: "[1,2,3,4,5,6]",
				},
			},
			false,
			false,
		},
		{
			"dynamic-fields-label-missing",
			func(p *Config) {
				p.HeaderAttribute = "Fields"
			},
			[]entry.Entry{
				{
					Body: "[\"stanza dev\",1,400,\"555-555-5555\"]",
				},
			},
			[]entry.Entry{
				{
					Body: map[string]interface{}{
						"name":   "stanza dev",
						"age":    int64(1),
						"height": int64(400),
						"number": "555-555-5555",
					},
				},
			},
			false,
			true,
		},
		{
			"missing-header-field",
			func(p *Config) {
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",1,400,\"555-555-5555\"]",
				},
			},
			[]entry.Entry{
				{
					Body: map[string]interface{}{
						"name":   "stanza",
						"age":    int64(1),
						"height": int64(400),
						"number": "555-555-5555",
					},
				},
			},
			true,
			false,
		},
		{
			"empty field",
			func(p *Config) {
				p.Header = "name,address,age,phone,position"
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"Evergreen\",,\"555-5555\",\"agent\"]",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":     "stanza",
						"address":  "Evergreen",
						"age":      "",
						"phone":    "555-5555",
						"position": "agent",
					},
					Body: "stanza,Evergreen,,555-5555,agent",
				},
			},
			false,
			true,
		},
		{
			"comma in quotes",
			func(p *Config) {
				p.Header = "name,address,age,phone,position"
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",\"Evergreen,49508\",1,\"555-5555\",\"agent\"]",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":     "stanza",
						"address":  "Evergreen,49508",
						"age":      int64(1),
						"phone":    "555-5555",
						"position": "agent",
					},
					Body: "[\"stanza\",\"Evergreen,49508\",1,\"555-5555\",\"agent\"]",
				},
			},
			false,
			false,
		},
		{
			"invalid-delimiter",
			func(p *Config) {
				// expect []rune of length 1
				p.Header = "name,,age,,height,,number"
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",1,400,\"555-555-5555\"]",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    int64(1),
						"height": int64(400),
						"number": "555-555-5555",
					},
					Body: "[\"stanza\",1,400,\"555-555-5555\"]",
				},
			},
			false,
			true,
		},
		{
			"invalid-header-delimiter",
			func(p *Config) {
				// expect []rune of length 1
				p.Header = "name,,age,,height,,number"
				p.HeaderDelimiter = ",,"
			},
			[]entry.Entry{
				{
					Body: "[\"stanza\",1,400,\"555-555-5555\"]",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    int64(1),
						"height": int64(400),
						"number": "555-555-5555",
					},
					Body: "[\"stanza\",1,400,\"555-555-5555\"]",
				},
			},
			true,
			false,
		},
		{
			"parse-failure-num-fields-mismatch",
			func(p *Config) {
				p.Header = "name,age,height,number"
			},
			[]entry.Entry{
				{
					Body: "[1,400,\"555-555-5555\"]",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    int64(1),
						"height": int64(400),
						"number": "555-555-5555",
					},
					Body: "[1,400,\"555-555-5555\"]",
				},
			},
			false,
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			op, err := cfg.Build(testutil.Logger(t))
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
	cases := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			"no_newlines",
			"[\"aaaa\",\"bbbb\",12,true,\"eeee\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": int64(12),
				"D": true,
				"E": "eeee",
			},
		},
		{
			"first_field",
			"[\"aa\naa\",\"bbbb\",12,true,\"eeee\"]",
			map[string]interface{}{
				"A": "aa\naa",
				"B": "bbbb",
				"C": int64(12),
				"D": true,
				"E": "eeee",
			},
		},
		{
			"middle_field",
			"[\"aaaa\",\"bb\nbb\",12,true,\"eeee\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bb\nbb",
				"C": int64(12),
				"D": true,
				"E": "eeee",
			},
		},
		{
			"last_field",
			"[\"aaaa\",\"bbbb\",12,true,\"e\neee\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": int64(12),
				"D": true,
				"E": "e\neee",
			},
		},
		{
			"multiple_fields",
			"[\"aaaa\",\"bb\nbb\",12,true,\"e\neee\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bb\nbb",
				"C": int64(12),
				"D": true,
				"E": "e\neee",
			},
		},
		{
			"multiple_first_field",
			"[\"a\na\na\na\",\"bbbb\",\"cccc\",\"dddd\",\"eeee\"]",
			map[string]interface{}{
				"A": "a\na\na\na",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"multiple_middle_field",
			"[\"aaaa\",\"bbbb\",\"c\nc\nc\nc\",\"dddd\",\"eeee\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "c\nc\nc\nc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"multiple_last_field",
			"[\"aaaa\",\"bbbb\",\"cccc\",\"dddd\",\"e\ne\ne\ne\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "e\ne\ne\ne",
			},
		},
		{
			"leading_newline",
			"[\"\naaaa\",\"bbbb\",\"cccc\",\"dddd\",\"eeee\"]",
			map[string]interface{}{
				"A": "\naaaa",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"trailing_newline",
			"[\"aaaa\",\"bbbb\",\"cccc\",\"dddd\",\"eeee\n\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "eeee\n",
			},
		},
		{
			"leading_newline_field",
			"[\"aaaa\",\"\nbbbb\",\"\ncccc\",\"\ndddd\",\"eeee\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "\nbbbb",
				"C": "\ncccc",
				"D": "\ndddd",
				"E": "eeee",
			},
		},
		{
			"trailing_newline_field",
			"[\"aaaa\",\"bbbb\n\",\"cccc\n\",\"dddd\n\",\"eeee\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb\n",
				"C": "cccc\n",
				"D": "dddd\n",
				"E": "eeee",
			},
		},
		{
			"empty_lines_unquoted",
			"[\"aa\naa\",\"bbbb\",\"c\nccc\",\"dddd\",\"eee\ne\"]",
			map[string]interface{}{
				"A": "aa\naa",
				"B": "bbbb",
				"C": "c\nccc",
				"D": "dddd",
				"E": "eee\ne",
			},
		},
		{
			"literal_return",
			`["aaaa","bb
bb","cccc","dd
dd","eeee"]`,
			map[string]interface{}{
				"A": "aaaa",
				"B": "bb\nbb",
				"C": "cccc",
				"D": "dd\ndd",
				"E": "eeee",
			},
		},
		{
			"return_in_quotes",
			"[\"aaaa\",\"bbbb\",\"cc\ncc\",\"dddd\",\"eeee\"]",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cc\ncc",
				"D": "dddd",
				"E": "eeee",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
			cfg.OutputIDs = []string{"fake"}
			cfg.Header = "A,B,C,D,E"

			op, err := cfg.Build(testutil.Logger(t))
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
	newBasicParser := func() *Config {
		cfg := NewConfigWithID("test")
		cfg.OutputIDs = []string{"test"}
		cfg.Header = "name,position,number"
		return cfg
	}

	t.Run("BasicConfig", func(t *testing.T) {
		c := newBasicParser()
		_, err := c.Build(testutil.Logger(t))
		require.NoError(t, err)
	})

	t.Run("MissingHeaderField", func(t *testing.T) {
		c := newBasicParser()
		c.Header = ""
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
	})

	t.Run("InvalidHeaderFieldMissingDelimiter", func(t *testing.T) {
		c := newBasicParser()
		c.Header = "name"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing field delimiter in header")
	})

	t.Run("InvalidHeaderFieldWrongDelimiter", func(t *testing.T) {
		c := newBasicParser()
		c.Header = "name;position;number"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
	})
}
