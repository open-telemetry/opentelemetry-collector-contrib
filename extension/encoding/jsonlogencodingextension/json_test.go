// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension

import (
	"bytes"
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
			inputReader := newStreamReader(bytes.NewReader([]byte(tt.input)))
			var inputDocuments []map[string]any
			var value map[string]any
			for inputReader.next() {
				value, err = inputReader.value()
				assert.NoError(t, err)
				inputDocuments = append(inputDocuments, value)
			}

			outputReader := newStreamReader(bytes.NewReader(buf))
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
