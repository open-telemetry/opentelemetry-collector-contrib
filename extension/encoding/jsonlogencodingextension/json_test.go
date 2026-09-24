// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

var testDataDir = "testdata"

func TestMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arrayMode bool
		input     string
		wantLogs  int
		logsPath  string
	}{
		{
			name:      "Array mode - single log",
			arrayMode: true,
			input:     `[{"example":"example valid json to test that the unmarshaler is correctly returning a plog value"}]`,
			wantLogs:  1,
			logsPath:  filepath.Join(testDataDir, "array_mode_single_log.yml"),
		},
		{
			name:      "Array mode - multiple logs",
			arrayMode: true,
			input:     `[{"example":"example valid json to test that the unmarshaler is correctly returning a plog value"}, {"key": "value"}]`,
			wantLogs:  2,
			logsPath:  filepath.Join(testDataDir, "array_mode_multi_log.yml"),
		},
		{
			name:      "JSON mode - single log pretty print",
			arrayMode: false,
			input: `{
					  "key-string": "value",
					  "key-int": 123456789,
					  "key-boolean": true
					}`,
			wantLogs: 1,
			logsPath: filepath.Join(testDataDir, "json_mode_single_log.yml"),
		},
		{
			name:      "JSON mode - new line delimited logs",
			arrayMode: false,
			input:     "{\"key-string\": \"value\",\"key-int\": 123456789,\"key-boolean\": true}\n{\"key-string\": \"value\",\"key-int\": 987654321,\"key-boolean\": false}",
			wantLogs:  2,
			logsPath:  filepath.Join(testDataDir, "json_mode_ndjson_log.yml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &jsonLogExtension{
				config: &Config{
					Mode:      JSONEncodingModeBody,
					ArrayMode: tt.arrayMode,
				},
			}

			logs, err := e.UnmarshalLogs([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, tt.wantLogs, logs.LogRecordCount())

			expected, err := golden.ReadLogs(tt.logsPath)
			assert.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expected, logs))

			buf, err := e.MarshalLogs(logs)
			assert.NoError(t, err)
			assert.NotEmpty(t, buf)

			if tt.arrayMode {
				assert.JSONEq(t, tt.input, string(buf))
				return
			}

			// special comparison for non array JSON. Compared in decoded format.
			inputReader := newStreamReader(bytes.NewReader([]byte(tt.input)), false)
			var inputDocuments []map[string]any
			var value map[string]any
			for inputReader.next() {
				value, err = inputReader.value()
				assert.NoError(t, err)
				inputDocuments = append(inputDocuments, value)
			}

			outputReader := newStreamReader(bytes.NewReader(buf), false)
			var outputDocuments []map[string]any
			for outputReader.next() {
				value, err = outputReader.value()
				assert.NoError(t, err)
				outputDocuments = append(outputDocuments, value)
			}

			require.NoError(t, err)
			for i, line := range inputDocuments {
				assert.Equal(t, line, outputDocuments[i])
			}
		})
	}
}

func TestUnmarshalLogsParseInts(t *testing.T) {
	tests := []struct {
		name      string
		arrayMode bool
		input     string
	}{
		{
			name:      "array mode",
			arrayMode: true,
			input:     `[{"small":1,"large":9007199254740993,"max":9223372036854775807,"min":-9223372036854775808,"decimal":1.0,"fraction":1.5,"exponent":1e3,"nested":{"id":9007199254740993},"array":[9007199254740993,1.5]}]`,
		},
		{
			name:      "JSON mode",
			arrayMode: false,
			input:     `{"small":1,"large":9007199254740993,"max":9223372036854775807,"min":-9223372036854775808,"decimal":1.0,"fraction":1.5,"exponent":1e3,"nested":{"id":9007199254740993},"array":[9007199254740993,1.5]}`,
		},
	}

	expected := map[string]any{
		"small":    int64(1),
		"large":    int64(9007199254740993),
		"max":      int64(9223372036854775807),
		"min":      int64(-9223372036854775808),
		"decimal":  float64(1),
		"fraction": 1.5,
		"exponent": float64(1000),
		"nested": map[string]any{
			"id": int64(9007199254740993),
		},
		"array": []any{int64(9007199254740993), 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extension := &jsonLogExtension{
				config: &Config{
					Mode:      JSONEncodingModeBody,
					ArrayMode: tt.arrayMode,
					ParseInts: true,
				},
			}

			logs, err := extension.UnmarshalLogs([]byte(tt.input))
			require.NoError(t, err)
			require.Equal(t, 1, logs.LogRecordCount())
			body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()
			assert.Equal(t, expected, body)
		})
	}
}

func TestUnmarshalLogsParseIntsDisabled(t *testing.T) {
	extension := &jsonLogExtension{
		config: &Config{
			Mode:      JSONEncodingModeBody,
			ArrayMode: false,
		},
	}

	logs, err := extension.UnmarshalLogs([]byte(`{"large":9007199254740993}`))
	require.NoError(t, err)
	require.Equal(t, 1, logs.LogRecordCount())
	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()
	assert.Equal(t, float64(9007199254740992), body["large"])
}

func BenchmarkUnmarshalLogs(b *testing.B) {
	const record = `{"small":1,"large":9007199254740993,"decimal":1.0,"nested":{"id":9007199254740993},"array":[9007199254740993,1.5]}`
	for _, arrayMode := range []bool{false, true} {
		input := []byte(record)
		if arrayMode {
			input = []byte("[" + record + "]")
		}
		for _, parseInts := range []bool{false, true} {
			b.Run(fmt.Sprintf("array_mode=%t/parse_ints=%t", arrayMode, parseInts), func(b *testing.B) {
				extension := &jsonLogExtension{
					config: &Config{
						Mode:      JSONEncodingModeBody,
						ArrayMode: arrayMode,
						ParseInts: parseInts,
					},
				}

				b.ReportAllocs()
				for b.Loop() {
					if _, err := extension.UnmarshalLogs(input); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func TestUnmarshalLogsArrayValidation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantLogs int
	}{
		{name: "array with whitespace", input: " \t\r\n[{\"id\":1}] \t\r\n", wantLogs: 1},
		{name: "empty array", input: `[]`},
		{name: "null", input: `null`},
		{name: "leading comma", input: `,[{"id":1}]`, wantErr: true},
		{name: "leading colon", input: `:[{"id":1}]`, wantErr: true},
		{name: "whitespace before comma", input: " \t\r\n,[{\"id\":1}]", wantErr: true},
		{name: "whitespace before colon", input: " \t\r\n:[{\"id\":1}]", wantErr: true},
		{name: "second array", input: `[{"id":1}][{"id":2}]`, wantErr: true},
		{name: "trailing null", input: `[{"id":1}] null`, wantErr: true},
		{name: "trailing comma", input: `[{"id":1}],`, wantErr: true},
		{name: "truncated array", input: `[{"id":1}`, wantErr: true},
		{name: "object instead of array", input: `{"id":1}`, wantErr: true},
		{name: "empty input", input: "", wantErr: true},
	}
	for _, parseInts := range []bool{false, true} {
		t.Run(fmt.Sprintf("parse_ints=%t", parseInts), func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					extension := &jsonLogExtension{config: &Config{
						Mode:      JSONEncodingModeBody,
						ArrayMode: true,
						ParseInts: parseInts,
					}}
					logs, err := extension.UnmarshalLogs([]byte(tt.input))
					if tt.wantErr {
						require.Error(t, err)
						return
					}
					require.NoError(t, err)
					assert.Equal(t, tt.wantLogs, logs.LogRecordCount())
				})
			}
		})
	}
}

func TestUnmarshalLogsParseIntsLimits(t *testing.T) {
	const record = `{"integer":9007199254740993,"exponent":9007199254740993e0,"decimal":9007199254740993.0,"overflow":9223372036854775809,"underflow":-9223372036854775809}`
	expected := map[string]any{
		"integer":   int64(9007199254740993),
		"exponent":  float64(9007199254740992),
		"decimal":   float64(9007199254740992),
		"overflow":  float64(9223372036854775808),
		"underflow": float64(-9223372036854775808),
	}
	for _, arrayMode := range []bool{false, true} {
		t.Run(fmt.Sprintf("array_mode=%t", arrayMode), func(t *testing.T) {
			extension := &jsonLogExtension{config: &Config{
				Mode:      JSONEncodingModeBody,
				ArrayMode: arrayMode,
				ParseInts: true,
			}}
			input := record + "\n" + record
			if arrayMode {
				input = "[" + record + "," + record + "]"
			}
			logs, err := extension.UnmarshalLogs([]byte(input))
			require.NoError(t, err)
			require.Equal(t, 2, logs.LogRecordCount())
			records := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
			for i := 0; i < records.Len(); i++ {
				assert.Equal(t, expected, records.At(i).Body().Map().AsRaw())
			}

			input = `{"overflow":1e400}`
			if arrayMode {
				input = "[" + input + "]"
			}
			_, err = extension.UnmarshalLogs([]byte(input))
			require.Error(t, err)
		})
	}
}

func TestInvalidMarshal(t *testing.T) {
	e := &jsonLogExtension{
		config: &Config{
			Mode: JSONEncodingModeBody,
		},
	}
	p := plog.NewLogs()
	p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("NOT A MAP")
	_, err := e.MarshalLogs(p)
	assert.ErrorContains(t, err, "marshal: expected 'Map' found 'Str'")
}

func TestInvalidUnmarshal(t *testing.T) {
	e := &jsonLogExtension{
		config: &Config{
			Mode:      JSONEncodingModeBody,
			ArrayMode: true,
		},
	}
	_, err := e.UnmarshalLogs([]byte("NOT A JSON"))
	assert.ErrorContains(t, err, "json: slice unexpected end of JSON input")
}

func TestPrettyLogProcessor(t *testing.T) {
	j := &jsonLogExtension{
		config: &Config{
			Mode:      JSONEncodingModeBodyWithInlineAttributes,
			ArrayMode: true,
		},
	}
	lp, err := j.MarshalLogs(sampleLog())
	assert.NoError(t, err)
	assert.NotNil(t, lp)
	assert.JSONEq(t, `[{"body":{"log":"test"},"logAttributes":{"foo":"bar"},"resourceAttributes":{"test":"logs-test"}},{"body":"log testing","resourceAttributes":{"test":"logs-test"}}]`, string(lp))
}

func sampleLog() plog.Logs {
	l := plog.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("test", "logs-test")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetEmptyMap().PutStr("log", "test")
	rl.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo", "bar")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log testing")
	return l
}
