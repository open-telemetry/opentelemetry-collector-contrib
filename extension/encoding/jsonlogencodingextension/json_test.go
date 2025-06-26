// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		decodingMode ProcessingMode
		input        string
		wantLogs     int
	}{
		{
			name:         "Array mode - single log",
			decodingMode: ArrayMode,
			input:        `[{"example":"example valid json to test that the unmarshaler is correctly returning a plog value"}]`,
			wantLogs:     1,
		},
		{
			name:         "Array mode - multiple logs",
			decodingMode: ArrayMode,
			input:        `[{"example":"example valid json to test that the unmarshaler is correctly returning a plog value"}, {"key": "value"}]`,
			wantLogs:     2,
		},
		{
			name:         "JSON mode - single log pretty print",
			decodingMode: JSONMode,
			input: `{
					  "key-string": "value",
					  "key-int": 123456789,
					  "key-boolean": true
					}`,
			wantLogs: 1,
		},
		{
			name:         "JSON mode - new line delimited logs",
			decodingMode: JSONMode,
			input:        "{\"key-string\": \"value\",\"key-int\": 123456789,\"key-boolean\": true}\n{\"key-string\": \"value\",\"key-int\": 987654321,\"key-boolean\": false}",
			wantLogs:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &jsonLogExtension{
				config: &Config{
					Mode:           JSONEncodingModeBody,
					ProcessingMode: tt.decodingMode,
				},
			}

			ld, err := e.UnmarshalLogs([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, tt.wantLogs, ld.LogRecordCount())

			buf, err := e.MarshalLogs(ld)
			assert.NoError(t, err)
			assert.NotEmpty(t, buf)

			if tt.decodingMode == ArrayMode {
				assert.JSONEq(t, tt.input, string(buf))
				return
			}

			// special comparison for JSON. Compared in decoded format.
			inputDocuments, err := todDecodedJSONDocuments(bytes.NewReader([]byte(tt.input)))
			require.NoError(t, err)

			outputDocuments, err := todDecodedJSONDocuments(bytes.NewReader(buf))
			require.NoError(t, err)

			assert.Len(t, len(inputDocuments), len(outputDocuments))
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
			Mode: JSONEncodingModeBody,
		},
	}
	_, err := e.UnmarshalLogs([]byte("NOT A JSON"))
	assert.ErrorContains(t, err, "json: slice unexpected end of JSON input")
}

func TestPrettyLogProcessor(t *testing.T) {
	j := &jsonLogExtension{
		config: &Config{
			Mode: JSONEncodingModeBodyWithInlineAttributes,
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
