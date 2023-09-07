// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package csv

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

var testHeader = "name,sev,msg"

func newTestParser(t *testing.T) *Parser {
	cfg := NewConfigWithID("test")
	cfg.Header = testHeader
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*Parser)
}

func newTestParserIgnoreQuotes(t *testing.T) *Parser {
	cfg := NewConfigWithID("test")
	cfg.Header = testHeader
	cfg.IgnoreQuotes = true
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

func TestParserBuildFailureLazyIgnoreQuotes(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.Header = testHeader
	cfg.LazyQuotes = true
	cfg.IgnoreQuotes = true
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "only one of 'ignore_quotes' or 'lazy_quotes' can be true")
}

func TestParserBuildFailureInvalidDelimiter(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.Header = testHeader
	cfg.FieldDelimiter = ";;"
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid 'delimiter': ';;'")
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
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong number of fields: expected 3, found 1")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong number of fields: expected 3, found 1")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as csv")
}

func TestParserInvalidTypeIgnoreQuotes(t *testing.T) {
	parser := newTestParserIgnoreQuotes(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as csv")
}

func TestParserCSV(t *testing.T) {
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
					Body: "stanza,INFO,started agent",
				},
			},
			[]entry.Entry{
				{
					Body: "stanza,INFO,started agent",
					Attributes: map[string]interface{}{
						"name": "stanza",
						"sev":  "INFO",
						"msg":  "started agent",
					},
				},
			},
			false,
			false,
		},
		{
			"basic-different-delimiters",
			func(p *Config) {
				p.Header = testHeader
				p.HeaderDelimiter = ","
				p.FieldDelimiter = "|"
			},
			[]entry.Entry{
				{
					Body: "stanza|INFO|started agent",
				},
			},
			[]entry.Entry{
				{
					Body: "stanza|INFO|started agent",
					Attributes: map[string]interface{}{
						"name": "stanza",
						"sev":  "INFO",
						"msg":  "started agent",
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
					Body: "stanza,INFO,started agent",
				},
				{
					Body: "stanza,ERROR,agent killed",
				},
				{
					Body: "kernel,TRACE,oom",
				},
			},
			[]entry.Entry{
				{
					Body: "stanza,INFO,started agent",
					Attributes: map[string]interface{}{
						"name": "stanza",
						"sev":  "INFO",
						"msg":  "started agent",
					},
				},
				{
					Body: "stanza,ERROR,agent killed",
					Attributes: map[string]interface{}{
						"name": "stanza",
						"sev":  "ERROR",
						"msg":  "agent killed",
					},
				},
				{
					Body: "kernel,TRACE,oom",
					Attributes: map[string]interface{}{
						"name": "kernel",
						"sev":  "TRACE",
						"msg":  "oom",
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
				p.FieldDelimiter = ";"
			},
			[]entry.Entry{
				{
					Body: "stanza;Evergreen;1;555-5555;agent",
				},
			},
			[]entry.Entry{
				{
					Body: "stanza;Evergreen;1;555-5555;agent",
					Attributes: map[string]interface{}{
						"name":     "stanza",
						"address":  "Evergreen",
						"age":      "1",
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
				p.FieldDelimiter = ","
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name,age,height,number",
					},
					Body: "stanza dev,1,400,555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name,age,height,number",
						"name":   "stanza dev",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
					Body: "stanza dev,1,400,555-555-5555",
				},
			},
			false,
			false,
		},
		{
			"dynamic-fields-header-delimiter",
			func(p *Config) {
				p.HeaderAttribute = "Fields"
				p.FieldDelimiter = ","
				p.HeaderDelimiter = "|"
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name|age|height|number",
					},
					Body: "stanza dev,1,400,555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name|age|height|number",
						"name":   "stanza dev",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
					Body: "stanza dev,1,400,555-555-5555",
				},
			},
			false,
			false,
		},
		{
			"dynamic-fields-multiple-entries",
			func(p *Config) {
				p.HeaderAttribute = "Fields"
				p.FieldDelimiter = ","
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name,age,height,number",
					},
					Body: "stanza dev,1,400,555-555-5555",
				},
				{
					Attributes: map[string]interface{}{
						"Fields": "x,y",
					},
					Body: "000100,2",
				},
				{
					Attributes: map[string]interface{}{
						"Fields": "a,b,c,d,e,f",
					},
					Body: "1,2,3,4,5,6",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"Fields": "name,age,height,number",
						"name":   "stanza dev",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
					Body: "stanza dev,1,400,555-555-5555",
				},
				{
					Attributes: map[string]interface{}{
						"Fields": "x,y",
						"x":      "000100",
						"y":      "2",
					},
					Body: "000100,2",
				},
				{
					Attributes: map[string]interface{}{
						"Fields": "a,b,c,d,e,f",
						"a":      "1",
						"b":      "2",
						"c":      "3",
						"d":      "4",
						"e":      "5",
						"f":      "6",
					},
					Body: "1,2,3,4,5,6",
				},
			},
			false,
			false,
		},
		{
			"dynamic-fields-tab",
			func(p *Config) {
				p.HeaderAttribute = "columns"
				p.FieldDelimiter = "\t"
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"columns": "name	age	height	number",
					},
					Body: "stanza dev	1	400	555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Attributes: func() map[string]interface{} {
						m := map[string]interface{}{
							"name":   "stanza dev",
							"age":    "1",
							"height": "400",
							"number": "555-555-5555",
						}
						m["columns"] = "name	age	height	number"
						return m
					}(),
					Body: "stanza dev	1	400	555-555-5555",
				},
			},
			false,
			false,
		},
		{
			"dynamic-fields-label-missing",
			func(p *Config) {
				p.HeaderAttribute = "Fields"
				p.FieldDelimiter = ","
			},
			[]entry.Entry{
				{
					Body: "stanza dev,1,400,555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Body: map[string]interface{}{
						"name":   "stanza dev",
						"age":    "1",
						"height": "400",
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
				p.FieldDelimiter = ","
			},
			[]entry.Entry{
				{
					Body: "stanza,1,400,555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Body: map[string]interface{}{
						"name":   "stanza",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
				},
			},
			true,
			false,
		},
		{
			"mariadb-audit-log",
			func(p *Config) {
				p.Header = "timestamp,serverhost,username,host,connectionid,queryid,operation,database,object,retcode"
			},
			[]entry.Entry{
				{
					Body: "20210316 17:08:01,oiq-int-mysql,load,oiq-int-mysql.bluemedora.localnet,5,0,DISCONNECT,,,0",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"timestamp":    "20210316 17:08:01",
						"serverhost":   "oiq-int-mysql",
						"username":     "load",
						"host":         "oiq-int-mysql.bluemedora.localnet",
						"connectionid": "5",
						"queryid":      "0",
						"operation":    "DISCONNECT",
						"database":     "",
						"object":       "",
						"retcode":      "0",
					},
					Body: "20210316 17:08:01,oiq-int-mysql,load,oiq-int-mysql.bluemedora.localnet,5,0,DISCONNECT,,,0",
				},
			},
			false,
			false,
		},
		{
			"empty field",
			func(p *Config) {
				p.Header = "name,address,age,phone,position"
			},
			[]entry.Entry{
				{
					Body: "stanza,Evergreen,,555-5555,agent",
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
			false,
		},
		{
			"tab delimiter",
			func(p *Config) {
				p.Header = "name	address	age	phone	position"
				p.FieldDelimiter = "\t"
			},
			[]entry.Entry{
				{
					Body: "stanza	Evergreen	1	555-5555	agent",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":     "stanza",
						"address":  "Evergreen",
						"age":      "1",
						"phone":    "555-5555",
						"position": "agent",
					},
					Body: "stanza	Evergreen	1	555-5555	agent",
				},
			},
			false,
			false,
		},
		{
			"comma in quotes",
			func(p *Config) {
				p.Header = "name,address,age,phone,position"
			},
			[]entry.Entry{
				{
					Body: "stanza,\"Evergreen,49508\",1,555-5555,agent",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":     "stanza",
						"address":  "Evergreen,49508",
						"age":      "1",
						"phone":    "555-5555",
						"position": "agent",
					},
					Body: "stanza,\"Evergreen,49508\",1,555-5555,agent",
				},
			},
			false,
			false,
		},
		{
			"quotes in quotes",
			func(p *Config) {
				p.Header = "name,address,age,phone,position"
			},
			[]entry.Entry{
				{
					Body: "\"bob \"\"the man\"\"\",Evergreen,1,555-5555,agent",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":     "bob \"the man\"",
						"address":  "Evergreen",
						"age":      "1",
						"phone":    "555-5555",
						"position": "agent",
					},
					Body: "\"bob \"\"the man\"\"\",Evergreen,1,555-5555,agent",
				},
			},
			false,
			false,
		},
		{
			"missing-header-delimiter-in-header",
			func(p *Config) {
				p.Header = "name:age:height:number"
				p.FieldDelimiter = ","
			},
			[]entry.Entry{
				{
					Body: "stanza,1,400,555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
					Body: "stanza,1,400,555-555-5555",
				},
			},
			true,
			false,
		},
		{
			"invalid-delimiter",
			func(p *Config) {
				// expect []rune of length 1
				p.Header = "name,,age,,height,,number"
				p.FieldDelimiter = ",,"
			},
			[]entry.Entry{
				{
					Body: "stanza,1,400,555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
					Body: "stanza,1,400,555-555-5555",
				},
			},
			true,
			false,
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
					Body: "stanza,1,400,555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
					Body: "stanza,1,400,555-555-5555",
				},
			},
			true,
			false,
		},
		{
			"parse-failure-num-fields-mismatch",
			func(p *Config) {
				p.Header = "name,age,height,number"
				p.FieldDelimiter = ","
			},
			[]entry.Entry{
				{
					Body: "1,400,555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
					Body: "1,400,555-555-5555",
				},
			},
			false,
			true,
		},
		{
			"parse-failure-wrong-field-delimiter",
			func(p *Config) {
				p.Header = "name,age,height,number"
				p.FieldDelimiter = ","
			},
			[]entry.Entry{
				{
					Body: "stanza:1:400:555-555-5555",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza",
						"age":    "1",
						"height": "400",
						"number": "555-555-5555",
					},
				},
			},
			false,
			true,
		},
		{
			"parse-with-lazy-quotes",
			func(p *Config) {
				p.Header = "name,age,height,number"
				p.FieldDelimiter = ","
				p.LazyQuotes = true
			},
			[]entry.Entry{
				{
					Body: "stanza \"log parser\",1,6ft,5",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza \"log parser\"",
						"age":    "1",
						"height": "6ft",
						"number": "5",
					},
					Body: "stanza \"log parser\",1,6ft,5",
				},
			},
			false,
			false,
		},
		{
			"parse-with-ignore-quotes",
			func(p *Config) {
				p.Header = "name,age,height,number"
				p.FieldDelimiter = ","
				p.IgnoreQuotes = true
			},
			[]entry.Entry{
				{
					Body: "stanza log parser,1,6ft,5",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza log parser",
						"age":    "1",
						"height": "6ft",
						"number": "5",
					},
					Body: "stanza log parser,1,6ft,5",
				},
			},
			false,
			false,
		},
		{
			"parse-with-ignore-quotes-bytes",
			func(p *Config) {
				p.Header = "name,age,height,number"
				p.FieldDelimiter = ","
				p.IgnoreQuotes = true
			},
			[]entry.Entry{
				{
					Body: []byte("stanza log parser,1,6ft,5"),
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza log parser",
						"age":    "1",
						"height": "6ft",
						"number": "5",
					},
					Body: []byte("stanza log parser,1,6ft,5"),
				},
			},
			false,
			false,
		},
		{
			"parse-with-ignore-quotes-invalid-csv",
			func(p *Config) {
				p.Header = "name,age,height,number"
				p.FieldDelimiter = ","
				p.IgnoreQuotes = true
			},
			[]entry.Entry{
				{
					Body: "stanza log parser,\"1,\"6ft,5\"",
				},
			},
			[]entry.Entry{
				{
					Attributes: map[string]interface{}{
						"name":   "stanza log parser",
						"age":    "\"1",
						"height": "\"6ft",
						"number": "5\"",
					},
					Body: "stanza log parser,\"1,\"6ft,5\"",
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

func TestParserCSVMultiline(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			"no_newlines",
			"aaaa,bbbb,cccc,dddd,eeee",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"first_field",
			"aa\naa,bbbb,cccc,dddd,eeee",
			map[string]interface{}{
				"A": "aa\naa",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"middle_field",
			"aaaa,bbbb,cc\ncc,dddd,eeee",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cc\ncc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"last_field",
			"aaaa,bbbb,cccc,dddd,e\neee",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "e\neee",
			},
		},
		{
			"multiple_fields",
			"aaaa,bb\nbb,ccc\nc,dddd,e\neee",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bb\nbb",
				"C": "ccc\nc",
				"D": "dddd",
				"E": "e\neee",
			},
		},
		{
			"multiple_first_field",
			"a\na\na\na,bbbb,cccc,dddd,eeee",
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
			"aaaa,bbbb,c\nc\nc\nc,dddd,eeee",
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
			"aaaa,bbbb,cccc,dddd,e\ne\ne\ne",
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
			"\naaaa,bbbb,cccc,dddd,eeee",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"trailing_newline",
			"aaaa,bbbb,cccc,dddd,eeee\n",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cccc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"leading_newline_field",
			"aaaa,\nbbbb,\ncccc,\ndddd,eeee",
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
			"aaaa,bbbb\n,cccc\n,dddd\n,eeee",
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
			"aa\n\naa,bbbb,c\n\nccc,dddd,eee\n\ne",
			map[string]interface{}{
				"A": "aa\naa",
				"B": "bbbb",
				"C": "c\nccc",
				"D": "dddd",
				"E": "eee\ne",
			},
		},
		{
			"empty_lines_quoted",
			"\"aa\n\naa\",bbbb,\"c\n\nccc\",dddd,\"eee\n\ne\"",
			map[string]interface{}{
				"A": "aa\n\naa",
				"B": "bbbb",
				"C": "c\n\nccc",
				"D": "dddd",
				"E": "eee\n\ne",
			},
		},
		{
			"everything",
			"\n\na\na\n\naa,\n\nbb\nbb\n\n,\"cc\ncc\n\n\",\ndddd\n,eeee\n\n",
			map[string]interface{}{
				"A": "a\na\naa",
				"B": "\nbb\nbb\n",
				"C": "cc\ncc\n\n",
				"D": "\ndddd\n",
				"E": "eeee",
			},
		},
		{
			"literal_return",
			`aaaa,bb
bb,cccc,dd
dd,eeee`,
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
			"aaaa,\"bbbb\",\"cc\ncc\",dddd,eeee",
			map[string]interface{}{
				"A": "aaaa",
				"B": "bbbb",
				"C": "cc\ncc",
				"D": "dddd",
				"E": "eeee",
			},
		},
		{
			"return_in_double_quotes",
			`aaaa,"""bbbb""","""cc
cc""",dddd,eeee`,
			map[string]interface{}{
				"A": "aaaa",
				"B": "\"bbbb\"",
				"C": "\"cc\ncc\"",
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

func TestParserCSVInvalidJSONInput(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		cfg := NewConfigWithID("test")
		cfg.OutputIDs = []string{"fake"}
		cfg.Header = testHeader

		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)

		fake := testutil.NewFakeOutput(t)
		require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

		entry := entry.New()
		entry.Body = "{\"name\": \"stanza\"}"
		err = op.Process(context.Background(), entry)
		require.Error(t, err, "parse error on line 1, column 1: bare \" in non-quoted-field")
		fake.ExpectBody(t, "{\"name\": \"stanza\"}")
	})
}

func TestBuildParserCSV(t *testing.T) {
	newBasicParser := func() *Config {
		cfg := NewConfigWithID("test")
		cfg.OutputIDs = []string{"test"}
		cfg.Header = "name,position,number"
		cfg.FieldDelimiter = ","
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

	t.Run("InvalidDelimiter", func(t *testing.T) {
		c := newBasicParser()
		c.Header = "name,position,number"
		c.FieldDelimiter = ":"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing field delimiter in header")
	})
}
