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
package csv

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

var testHeader = "name,sev,msg"

func newTestParser(t *testing.T) *CSVParser {
	cfg := NewCSVParserConfig("test")
	cfg.Header = testHeader
	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	op := ops[0]
	return op.(*CSVParser)
}

func TestCSVParserBuildFailure(t *testing.T) {
	cfg := NewCSVParserConfig("test")
	cfg.OnError = "invalid_on_error"
	_, err := cfg.Build(testutil.NewBuildContext(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestCSVParserBuildFailureInvalidDelimiter(t *testing.T) {
	cfg := NewCSVParserConfig("test")
	cfg.Header = testHeader
	cfg.FieldDelimiter = ";;"
	_, err := cfg.Build(testutil.NewBuildContext(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid 'delimiter': ';;'")
}

func TestCSVParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]uint8' cannot be parsed as csv")
}

func TestCSVParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "record on line 1: wrong number of fields")
}

func TestCSVParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as csv")
}

func TestParserCSV(t *testing.T) {
	cases := []struct {
		name       string
		configure  func(*CSVParserConfig)
		inputBody  interface{}
		outputBody interface{}
	}{
		{
			"basic",
			func(p *CSVParserConfig) {
				p.Header = testHeader
			},
			"stanza,INFO,started agent",
			map[string]interface{}{
				"name": "stanza",
				"sev":  "INFO",
				"msg":  "started agent",
			},
		},
		{
			"advanced",
			func(p *CSVParserConfig) {
				p.Header = "name;address;age;phone;position"
				p.FieldDelimiter = ";"
			},
			"stanza;Evergreen;1;555-5555;agent",
			map[string]interface{}{
				"name":     "stanza",
				"address":  "Evergreen",
				"age":      "1",
				"phone":    "555-5555",
				"position": "agent",
			},
		},
		{
			"mariadb-audit-log",
			func(p *CSVParserConfig) {
				p.Header = "timestamp,serverhost,username,host,connectionid,queryid,operation,database,object,retcode"
				tp := helper.NewTimeParser()
				field := entry.NewBodyField("timestamp")
				tp.ParseFrom = &field
				tp.LayoutType = "strptime"
				tp.Layout = "%Y%m%d"
				p.TimeParser = &tp
			},
			"20210316,oiq-int-mysql,load,oiq-int-mysql.bluemedora.localnet,5,0,DISCONNECT,,,0",
			map[string]interface{}{
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
		},
		{
			"empty field",
			func(p *CSVParserConfig) {
				p.Header = "name,address,age,phone,position"
			},
			"stanza,Evergreen,,555-5555,agent",
			map[string]interface{}{
				"name":     "stanza",
				"address":  "Evergreen",
				"age":      "",
				"phone":    "555-5555",
				"position": "agent",
			},
		},
		{
			"tab delimiter",
			func(p *CSVParserConfig) {
				p.Header = "name	address	age	phone	position"
				p.FieldDelimiter = "\t"
			},
			"stanza	Evergreen	1	555-5555	agent",
			map[string]interface{}{
				"name":     "stanza",
				"address":  "Evergreen",
				"age":      "1",
				"phone":    "555-5555",
				"position": "agent",
			},
		},
		{
			"comma in quotes",
			func(p *CSVParserConfig) {
				p.Header = "name,address,age,phone,position"
			},
			"stanza,\"Evergreen,49508\",1,555-5555,agent",
			map[string]interface{}{
				"name":     "stanza",
				"address":  "Evergreen,49508",
				"age":      "1",
				"phone":    "555-5555",
				"position": "agent",
			},
		},
		{
			"quotes in quotes",
			func(p *CSVParserConfig) {
				p.Header = "name,address,age,phone,position"
			},
			"\"bob \"\"the man\"\"\",Evergreen,1,555-5555,agent",
			map[string]interface{}{
				"name":     "bob \"the man\"",
				"address":  "Evergreen",
				"age":      "1",
				"phone":    "555-5555",
				"position": "agent",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewCSVParserConfig("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			ops, err := cfg.Build(testutil.NewBuildContext(t))
			require.NoError(t, err)
			op := ops[0]

			fake := testutil.NewFakeOutput(t)
			op.SetOutputs([]operator.Operator{fake})

			entry := entry.New()
			entry.Body = tc.inputBody
			err = op.Process(context.Background(), entry)
			require.NoError(t, err)
			if cfg.TimeParser != nil {
				newTime, _ := time.ParseInLocation("20060102", "20210316", entry.Timestamp.Location())
				require.Equal(t, newTime, entry.Timestamp)
			}
			fake.ExpectBody(t, tc.outputBody)
		})
	}
}

func TestParserCSVMultipleBodys(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		cfg := NewCSVParserConfig("test")
		cfg.OutputIDs = []string{"fake"}
		cfg.Header = testHeader

		ops, err := cfg.Build(testutil.NewBuildContext(t))
		require.NoError(t, err)
		op := ops[0]

		fake := testutil.NewFakeOutput(t)
		op.SetOutputs([]operator.Operator{fake})

		entry := entry.New()
		entry.Body = "stanza,INFO,started agent\nstanza,DEBUG,started agent"
		err = op.Process(context.Background(), entry)
		require.Nil(t, err, "Expected to parse a single csv record, got '2'")
		require.NoError(t, err)
		fake.ExpectBody(t, map[string]interface{}{
			"name": "stanza",
			"sev":  "INFO",
			"msg":  "started agent",
		})
	})
}

func TestParserCSVInvalidJSONInput(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		cfg := NewCSVParserConfig("test")
		cfg.OutputIDs = []string{"fake"}
		cfg.Header = testHeader

		ops, err := cfg.Build(testutil.NewBuildContext(t))
		require.NoError(t, err)
		op := ops[0]

		fake := testutil.NewFakeOutput(t)
		op.SetOutputs([]operator.Operator{fake})

		entry := entry.New()
		entry.Body = "{\"name\": \"stanza\"}"
		err = op.Process(context.Background(), entry)
		require.Nil(t, err, "parse error on line 1, column 1: bare \" in non-quoted-field")
		fake.ExpectBody(t, "{\"name\": \"stanza\"}")
	})
}

func TestBuildParserCSV(t *testing.T) {
	newBasicCSVParser := func() *CSVParserConfig {
		cfg := NewCSVParserConfig("test")
		cfg.OutputIDs = []string{"test"}
		cfg.Header = "name,position,number"
		cfg.FieldDelimiter = ","
		return cfg
	}

	t.Run("BasicConfig", func(t *testing.T) {
		c := newBasicCSVParser()
		_, err := c.Build(testutil.NewBuildContext(t))
		require.NoError(t, err)
	})

	t.Run("MissingHeaderField", func(t *testing.T) {
		c := newBasicCSVParser()
		c.Header = ""
		_, err := c.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
	})

	t.Run("InvalidHeaderFieldMissingDelimiter", func(t *testing.T) {
		c := newBasicCSVParser()
		c.Header = "name"
		_, err := c.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing field delimiter in header")
	})

	t.Run("InvalidHeaderFieldWrongDelimiter", func(t *testing.T) {
		c := newBasicCSVParser()
		c.Header = "name;position;number"
		_, err := c.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
	})

	t.Run("InvalidDelimiter", func(t *testing.T) {
		c := newBasicCSVParser()
		c.Header = "name,position,number"
		c.FieldDelimiter = ":"
		_, err := c.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing field delimiter in header")
	})
}
